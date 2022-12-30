package main

import (
	"fmt"
	"io"
	"math/rand"
	"net/http"
)

type Transfer struct {
	Body  io.ReadCloser
	Ready chan struct{}
}

func main() {
	transfers := map[string]*Transfer{}

	http.HandleFunc("/new-id", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Unknown method", http.StatusBadRequest)
			return
		}

		// FIXME: random seed
		id := fmt.Sprintf("%04X", rand.Uint64())

		// FIXME: thread safe
		transfers[id] = &Transfer{
			Body:  nil,
			Ready: make(chan struct{}),
		}

		w.Write([]byte(id))
	})

	http.HandleFunc("/file", func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		fileId := query.Get("fileId")

		if fileId == "" {
			http.Error(w, "Missing fileId parameter", http.StatusBadRequest)
			return
		}

		// FIXME: thread safe
		transfer, ok := transfers[fileId]

		if !ok {
			http.Error(w, "Unknown fileId", http.StatusBadRequest)
			return
		}

		// upload
		if r.Method == "POST" {
			// FIXME: thread safe
			transfer.Body = r.Body
			close(transfer.Ready)
			return
		}

		// download
		if r.Method == "GET" {
			<-transfer.Ready

			_, err := io.Copy(w, transfer.Body)
			if err != nil {
				// FIXME: "http: invalid Read on closed Body"
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
