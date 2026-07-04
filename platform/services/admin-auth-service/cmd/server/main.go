package main

import (
"fmt"
"log"
"net/http"
)

func main() {
	fmt.Println("🚀 Zippyra admin auth service starting...")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
