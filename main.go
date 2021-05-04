package main

import (
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

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
	var stoplock sync.Mutex
	stop := false
	stopChan := make(chan struct{}, 1)
	signalChan := make(chan os.Signal, 1)
	go func() {
		<-signalChan
		stoplock.Lock()
		stop = true
		stoplock.Unlock()
		log.Println("Terminating...")
		stopChan <- struct{}{}
		closeConn()
	}()
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	if err := dialdb(); err != nil {
		log.Fatalln("failed to dial MongoDB.: ", err)
	}
	defer closedb()

	// Channel for Votes result.
	votes := make(chan string)
	publisherStoppedChan := publishVotes(votes)
	twitterStoppedChan := startTwitterStream(stopChan, votes)
	go func() {
		for {
			time.Sleep(1 * time.Minute)
			closeConn()
			stoplock.Lock()
			if stop {
				stoplock.Unlock()
				break
			}
			stoplock.Unlock()
		}
	}()
	<-twitterStoppedChan
	close(votes)
	<-publisherStoppedChan
}
