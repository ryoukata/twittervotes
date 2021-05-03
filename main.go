package main

import (
	"log"

	"gopkg.in/mgo.v2"
)

// connect and close settings for MongoDB.
var db *mgo.Session

func dialdb() error {
	var err error
	log.Println("dialing to MongoDB...: localhost")
	db, err = mgo.Dial("localhost")

	return err
}
func closedb() {
	db.Close()
	log.Println("Closed Connection for MongoDB.")
}

// get all selection Strings to use poll.
type poll struct {
	Options []string
}

func loadOptions() ([]string, error) {
	var options []string
	iter := db.DB("ballots").C("polls").Find(nil).Iter()
	var p poll
	for iter.Next(&p) {
		options = append(options, p.Options...)
	}
	iter.Close()

	return options, iter.Err()
}

func main() {

}
