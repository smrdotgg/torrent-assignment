package main

import (
	"dod/torr/torrentfile"
	"os"
)

func main() {
	initial := os.Args[1]
	dest := "."

	tf, err := torrentfile.Open(initial)
	if err != nil {
		panic(err)
	}

	err = tf.Dl_to_dest(dest)
	if err != nil {
		panic(err)
	}
}
