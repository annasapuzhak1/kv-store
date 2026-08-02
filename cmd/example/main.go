package main

import (
	"fmt"
	"log"

	"github.com/annasapuzhak1/kv-store/client"
)

func main() {
	c := client.New("http://localhost:8080")

	// STORE
	key, err := c.Store("manual-test-1", []byte("name: John Doe"))
	if err != nil {
		log.Fatalf("Store failed: %v", err)
	}
	fmt.Println("Stored. Key:", key)

	// RETRIEVE
	data, err := c.Retrieve("manual-test-1", key)
	if err != nil {
		log.Fatalf("Retrieve failed: %v", err)
	}
	fmt.Println("Retrieved:", string(data))

	if string(data) != "name: John Doe" {
    	log.Fatalf("data mismatch: got %q, want %q", string(data), "name: John Doe")
	}
	fmt.Println("✓ Retrieved data matches what was stored")

	// UPDATE
	err = c.Update("manual-test-1", key, []byte("name: Jane Doe"))
	if err != nil {
		log.Fatalf("Update failed: %v", err)
	}
	fmt.Println("Updated successfully")

	// RETRIEVE again to confirm update took effect
	data, err = c.Retrieve("manual-test-1", key)
	if err != nil {
		log.Fatalf("Retrieve after update failed: %v", err)
	}
	fmt.Println("Retrieved after update:", string(data))

	if string(data) != "name: Jane Doe" {
		log.Fatalf("data mismatch after update: got %q, want %q", string(data), "name: Jane Doe")
	}
	fmt.Println("✓ Retrieved data matches what was updated")

	// DELETE
	err = c.Delete("manual-test-1", key)
	if err != nil {
		log.Fatalf("Delete failed: %v", err)
	}
	fmt.Println("Deleted successfully")

	// RETRIEVE again - should fail now
	_, err = c.Retrieve("manual-test-1", key)
	if err != nil {
		fmt.Println("Retrieve after delete correctly failed:", err)
	} else {
		fmt.Println("WARNING: retrieve after delete unexpectedly succeeded")
	}
}