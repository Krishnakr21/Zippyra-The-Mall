package main

import (
"fmt"
"log"
"net/http"
)

func main() {
	fmt.Println("🚀 Zippyra chain hq service starting...")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
