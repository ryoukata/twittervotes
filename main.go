package main

import (
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/joeshaw/envdecode"
	"github.com/nsqio/go-nsq"
	"gopkg.in/mgo.v2"
)

// connect and close settings for MongoDB.
var db *mgo.Session

func dialdb() error {
	var err error
	var mongoEnv struct {
		MongoHost   string `env:"MONGO_HOST,required"`
		MongoPort   string `env:"MONGO_PORT,required"`
		MongoDB     string `env:"MONGO_DB,required"`
		MongoUser   string `env:"MONGO_USER,required"`
		MongoPass   string `env:"MONGO_PASS,required"`
		MongoSource string `env:"MONGO_SOURCE,required"`
	}
	if err := envdecode.Decode(&mongoEnv); err != nil {
		log.Fatalln(err)
	}
	log.Println("dialing to MongoDB...: localhost")
	mongoInfo := &mgo.DialInfo{
		Addrs:    []string{mongoEnv.MongoHost + ":" + mongoEnv.MongoPort},
		Timeout:  20 * time.Second,
		Database: mongoEnv.MongoDB,
		Username: mongoEnv.MongoUser,
		Password: mongoEnv.MongoPass,
		Source:   mongoEnv.MongoSource,
	}
	db, err = mgo.DialWithInfo(mongoInfo)

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
	var nsqEnv struct {
		NsqHost  string `env:"NSQ_HOST,required"`
		NsqPort  string `env:"NSQ_PORT,required"`
		NsqTopic string `env:"NSQ_TOPIC,required"`
	}
	if err := envdecode.Decode(&nsqEnv); err != nil {
		log.Fatalln(err)
	}
	stopchan := make(chan struct{}, 1)
	pub, _ := nsq.NewProducer(nsqEnv.NsqHost+":"+nsqEnv.NsqPort, nsq.NewConfig())
	go func() {
		for vote := range votes {
			// publish vote result.
			pub.Publish(nsqEnv.NsqTopic, []byte(vote))
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
