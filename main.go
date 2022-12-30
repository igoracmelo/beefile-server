package main

import (
	"bufio"
	"fmt"
	"io"
	"math/rand"
	"net/http"
)

type Transfer struct {
	Body          io.ReadCloser
	Finished      chan struct{}
	ReadyToUpload chan struct{}
}

var transfers = map[string]*Transfer{}

func main() {
	http.HandleFunc("/new-id", handleNewId)
	http.HandleFunc("/upload", handleUpload)
	http.HandleFunc("/download", handleDownload)

	err := http.ListenAndServe(":1234", nil)
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

	// FIXME: thread safe
	transfers[id] = &Transfer{
		Body:          nil,
		Finished:      make(chan struct{}),
		ReadyToUpload: make(chan struct{}),
	}

	w.Write([]byte(id))
}

func handleDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, r.Method+" "+r.URL.Path, http.StatusBadRequest)
	}

	query := r.URL.Query()
	id := query.Get("id")

	if id == "" {
		http.Error(w, "Missing id parameter", http.StatusBadRequest)
		return
	}

	// FIXME: thread safe
	transfer, ok := transfers[id]

	if !ok {
		http.Error(w, "Unknown id", http.StatusNotFound)
		return
	}

	defer close(transfer.Finished)
	defer delete(transfers, id)
	<-transfer.ReadyToUpload

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
	if r.Method != "GET" {
		http.Error(w, r.Method+" "+r.URL.Path, http.StatusBadRequest)
	}

	query := r.URL.Query()
	id := query.Get("id")

	if id == "" {
		http.Error(w, "Missing id parameter", http.StatusBadRequest)
		return
	}

	// FIXME: thread safe
	transfer, ok := transfers[id]

	if !ok {
		http.Error(w, "Unknown id", http.StatusNotFound)
		return
	}

	transfer.Body = r.Body
	close(transfer.ReadyToUpload)
	<-transfer.Finished
}
