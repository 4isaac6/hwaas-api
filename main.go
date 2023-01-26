package main

import (
    "log"
    "net/http"
    "encoding/json"

    "github.com/gorilla/mux"
)

func home(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(struct { Message string } {
        Message: "Welcome to the Hello World API",
    })
}

func main() {
	router := mux.NewRouter()

    router.HandleFunc("/api", home).Methods("GET")

	log.Fatal(http.ListenAndServe(":8080", router))
}
