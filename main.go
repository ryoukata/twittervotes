package main

import (
	"log"

	"github.com/bitly/go-nsq"
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

// load choiecs from MongoDB.
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

// publish votes result to NSQ Topic.
func publishVotes(votes <-chan string) <-chan struct{} {
	stopchan := make(chan struct{}, 1)
	pub, _ := nsq.NewProducer("localhost:4150", nsq.NewConfig())
	go func() {
		for vote := range votes {
			// publish vote result.
			pub.Publish("votes", []byte(vote))
		}
		log.Println("Publisher: Terminating...")
		pub.Stop()
		log.Println("Publisher: Terminated.")
		stopchan <- struct{}{}
	}()
	return stopchan
}

func main() {

}
