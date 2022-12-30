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

func main() {
	transfers := map[string]*Transfer{}

	http.HandleFunc("/new-id", func(w http.ResponseWriter, r *http.Request) {
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
	})

	http.HandleFunc("/transfer", func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		transferId := query.Get("id")

		if transferId == "" {
			http.Error(w, "Missing fileId parameter", http.StatusBadRequest)
			return
		}

		// FIXME: thread safe
		transfer, ok := transfers[transferId]

		if !ok {
			http.Error(w, "Unknown fileId", http.StatusBadRequest)
			return
		}

		// upload
		if r.Method == "POST" {
			// FIXME: thread safe
			transfer.Body = r.Body
			close(transfer.ReadyToUpload)
			<-transfer.Finished
			return
		}

		// download
		if r.Method == "GET" {
			defer close(transfer.Finished)
			defer delete(transfers, transferId)
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

			return
		}

		http.Error(w, "Unknown method", http.StatusBadRequest)
	})

	err := http.ListenAndServe(":1234", nil)
	if err != nil {
		fmt.Println(err.Error())
	}
}
