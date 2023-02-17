package main

import (
	"bufio"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"sync"
)

type Transfer struct {
	Body        io.ReadCloser
	Finished    chan struct{}
	SenderReady chan struct{}
}

type Transfers struct {
	mu   *sync.Mutex
	data map[string]*Transfer
}

var transfers = Transfers{
	mu:   new(sync.Mutex),
	data: map[string]*Transfer{},
}

func main() {
	http.HandleFunc("/upload", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		http.ServeFile(w, r, "./pages/upload.html")
	})

	http.HandleFunc("/api/new-id", handleNewId)
	http.HandleFunc("/api/upload", handleUpload)
	http.HandleFunc("/download", handleDownload)
	http.HandleFunc("/api/download", handleDownload)

	port := os.Getenv("PORT")
	if port == "" {
		port = "12345"
	}

	err := http.ListenAndServe(":"+port, nil)
	if err != nil {
		fmt.Println(err.Error())
	}
}

func handleNewId(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Unknown method", http.StatusBadRequest)
		return
	}

	// FIXME: random seed
	id := fmt.Sprintf("%16X", rand.Uint64())
	id += fmt.Sprintf("%16X", rand.Uint64())

	transfers.mu.Lock()
	defer transfers.mu.Unlock()

	transfers.data[id] = &Transfer{
		Body:        nil,
		Finished:    make(chan struct{}),
		SenderReady: make(chan struct{}),
	}

	w.Write([]byte(id))
}

func handleDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, r.Method+" "+r.URL.Path, http.StatusBadRequest)
		return
	}

	query := r.URL.Query()
	id := query.Get("id")

	if id == "" {
		http.Error(w, "Missing id parameter", http.StatusBadRequest)
		return
	}

	transfers.mu.Lock()
	transfer, ok := transfers.data[id]
	transfers.mu.Unlock()

	if !ok {
		http.Error(w, "Unknown id", http.StatusNotFound)
		return
	}

	defer func() {
		close(transfer.Finished)

		transfers.mu.Lock()
		delete(transfers.data, id)
		transfers.mu.Unlock()
	}()

	<-transfer.SenderReady

	w.Header().Add("Content-Type", "application/octet-stream")
	w.Header().Add("Transfer-Encoding", "chunked")

	writer := bufio.NewWriter(w)
	_, err := io.Copy(writer, transfer.Body)
	if err != nil {
		http.Error(w, "Failed to download file: "+err.Error(), http.StatusInternalServerError)
		return
	}

	err = writer.Flush()
	if err != nil {
		http.Error(w, "Failed to download file: "+err.Error(), http.StatusInternalServerError)
		return
	}
}

func handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, r.Method+" "+r.URL.Path, http.StatusBadRequest)
		return
	}

	query := r.URL.Query()
	id := query.Get("id")

	if id == "" {
		http.Error(w, "Missing id parameter", http.StatusBadRequest)
		return
	}

	transfers.mu.Lock()
	transfer, ok := transfers.data[id]

	if !ok {
		transfers.mu.Unlock()
		http.Error(w, "Unknown id", http.StatusNotFound)
		return
	}

	transfer.Body = r.Body
	transfers.mu.Unlock()

	close(transfer.SenderReady)
	<-transfer.Finished
}
