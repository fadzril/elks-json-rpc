package main

import (
	JSON "encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
	"github.com/gorilla/rpc"
	"github.com/gorilla/rpc/json"
	"github.com/streadway/amqp"
	"github.com/zpatrick/go-config"
)

func init() {
	log.SetFormatter(&log.TextFormatter{})
}

func initConfig() *config.Config {
	iniFile := config.NewINIFile("config.ini")
	return config.NewConfig([]config.Provider{iniFile})
}

func main() {

	cfg := initConfig()
	if err := cfg.Load(); err != nil {
		log.Println(err)
	}

	// Environment Config
	host, _ := cfg.String("rabbit.host")
	port, _ := cfg.String("rabbit.port")
	user, _ := cfg.String("rabbit.user")
	pwd, _ := cfg.String("rabbit.pass")
	apis := &API{url: "amqp://" + user + ":" + pwd + "@" + host + ":" + port}
	log.Println("Connecting to RABBIT URI: ", apis.url)

	r := mux.NewRouter()
	s := rpc.NewServer()
	c := json.NewCodec()
	s.RegisterCodec(c, "application/json")
	s.RegisterService(new(API), "")
	r.Handle("/api", s)

	log.WithFields(log.Fields{
		"port": 8080,
	}).Info("Starting Web Server")

	log.Fatal(http.ListenAndServe(":8080", r))
}

// Client Struct
type Client struct {
	Queue   string `json:"queue"`
	Key     string `json:"key"`
	Message string `json:"message"`
	Service string `json:"service"`
}

// API struct
type API struct {
	url string
}

// Messages struct
type Messages struct {
	Tags      []string `json:"tags"`
	Version   string   `json:"@version"`
	Timestamp string   `json:"@timestamp"`
	Message   string   `json:"message"`
	Type      string   `json:"type"`
}

// SetupMQ ...
func SetupMQ(uri string) (ch *amqp.Channel, err error) {
	conn, err := amqp.Dial(uri)
	defer conn.Close()

	fmt.Println("Going Setup")

	if err == nil {
		ch, errors := conn.Channel()
		defer ch.Close()
		if errors != nil {
			failOnError(errors, "Error Opening Channell")
			return nil, errors
		}
		return ch, nil
	}
	return nil, err
}

func failOnError(err error, msg string) {
	if err != nil {
		log.WithFields(log.Fields{
			"error": true,
			"msg":   msg,
		}).Fatal("The ice breaks!")

		panic(fmt.Sprintf("%s: %s", msg, err))
	}
}

// GetMessages ...
func (api *API) GetMessages(r *http.Request, client *Client, reply *Client) error {

	mq, _ := amqp.Dial(api.url)
	defer mq.Close()

	s, e := mq.Channel()
	if e != nil {
		failOnError(e, "Error Getting Channel")
	}
	defer s.Close()

	q := strings.ToUpper(client.Service) + "-Q"
	forever := make(chan bool)

	m, e := s.Consume(
		q,
		"",
		false,
		false,
		false,
		false,
		nil,
	)
	if e != nil {
		fmt.Printf("Error Getting Messages: %v", e)
		return e
	}

	go func() {
		for {
			for msg := range m {
				o := fmt.Sprintf("Message: %s\n", string(msg.Body))
				reply.Message = o
				log.WithFields(log.Fields{
					"message": o,
					"queue":   msg,
				}).Info("Consume output")
			}
		}
	}()

	<-forever

	return nil
}

// SendMessage ...
func (api *API) SendMessage(r *http.Request, client *Client, reply *Client) error {

	x := "RPC-X"
	k := "RPC-K"
	q := "RPC-Q"

	t := func(c string) string {
		if strings.ToLower(c) == "tibco" {
			return "REALTIME"
		}
		return strings.ToUpper(c)
	}
	b := &Messages{
		Tags:      []string{strings.ToLower(client.Service)},
		Version:   "1.0",
		Timestamp: time.Now().Format(time.RFC3339),
		Message:   client.Message,
		Type:      t(client.Service),
	}

	mj, err := JSON.Marshal(b)

	if err != nil {
		failOnError(err, "Failed Marshalling")
		return err
	}

	fmt.Println("READY MAKE CONNECTION ", api.url, x)

	mq, _ := amqp.Dial(api.url)
	defer mq.Close()

	s, e := mq.Channel()
	if e != nil {
		failOnError(e, "Error Getting Channel")
	}
	defer s.Close()

	ers := s.Publish(
		x,
		k,
		false,
		false,
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        []byte(string(mj)),
		},
	)
	if ers != nil {
		failOnError(ers, "Error when try to publish message")
		return ers
	}

	reply.Queue = q
	reply.Key = k
	reply.Service = "RPC-JSON"
	reply.Message = "OK"

	log.WithFields(log.Fields{
		"reply": reply,
	}).Info("Request Completed!")
	return nil
}
