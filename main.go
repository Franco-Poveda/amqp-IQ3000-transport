package main

import (
	"encoding/hex"
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"github.com/jasonlvhit/gocron"
	"github.com/joho/godotenv"
	"github.com/streadway/amqp"
)

var amqpconn *amqp.Connection
var amqpch *amqp.Channel

//Message struct
type Message struct {
	Values []string
	Time   time.Time
}

func main() {
	var Env map[string]string
	Env, err := godotenv.Read()
	checkError(err)
	amqpconn, amqpch, _ = setup(Env)
	getIQvalues(Env)
	gocron.Every(1).Minute().Do(getIQvalues, Env)
	<-gocron.Start()
}

func getIQvalues(Env map[string]string) {
	conn, err := net.Dial("tcp", Env["IQ_TTY"])
	checkError(err)

	var c = []byte{0x0d, 0x0d, 0x0d}
	var d = []byte{0x49, 0x4e, 0x50, 0x55, 0x54, 0x56, 0x41, 0x4c, 0x0d}
	var e = []byte{0x45, 0x58, 0x49, 0x54, 0x0d}
	conn.Write(c)

	message, _ := bufio.NewReader(conn).ReadBytes(0x2a)
	fmt.Println("connected to IQ3000", len(message), message[:1])
	conn.Write(d)
	values, _ := bufio.NewReader(conn).ReadBytes(0x2a)
	fmt.Printf("%s", hex.Dump(values))

	raw := dropCR(values)
	sraw := string(raw)
	result := strings.Split(sraw, "\n\r")
	result = result[1 : len(result)-1]

	log.Println(len(result))

	for i := range result {
		fmt.Println(result[i])
	}
	msg := Message{result, time.Now()}

	resJSON, _ := json.Marshal(msg)
	err = amqpch.Publish(
		"",           // exchange
		Env["QUEUE"], // routing key
		false,        // mandatory
		false,        // immediate
		amqp.Publishing{
			ContentType:  "text/plain",
			Body:         []byte(string(resJSON)),
			DeliveryMode: amqp.Persistent,
			Timestamp:    time.Now(),
		})
	log.Printf(" [x] Sent %s", resJSON)
	checkError(err)

	time.Sleep(2 * time.Second)
	conn.Write(e)
	time.Sleep(2 * time.Second)
	conn.Close()
	fmt.Println("disconected from device")

}

func checkError(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "Fatal error: %s", err.Error())
		os.Exit(1)
	}
}

func dropCR(data []byte) []byte {
	if len(data) > 0 && data[len(data)-1] == 0x2d {
		return data[0 : len(data)-1]
	}
	return data
}

func setup(Env map[string]string) (*amqp.Connection, *amqp.Channel, error) {
	var err error
	amqpconn, err = amqp.Dial(Env["RABBIT_URI"])
	checkError(err)
	amqpch, err = amqpconn.Channel()
	checkError(err)
	_, err = amqpch.QueueDeclare(Env["QUEUE"], true, false, false, false, nil)
	checkError(err)
	return amqpconn, amqpch, nil
}
