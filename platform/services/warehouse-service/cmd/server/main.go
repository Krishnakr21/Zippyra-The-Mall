package main

import (
"fmt"
"log"
"net/http"
)

func main() {
	fmt.Println("�� Zippyra Warehouse Service starting...")
	log.Fatal(http.ListenAndServe(":8010", nil))
}
