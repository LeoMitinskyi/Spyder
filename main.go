package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	_ "github.com/lib/pq"
	"github.com/sevlyar/go-daemon"
	"log"
	"net"
	"os"
	"syscall"
	"time"
)

const (
	host     = "127.0.0.1"
	port     = 5432
	user     = "spyder"
	password = "spyder"
	dbname   = "tlc"
)

var (
	signal = flag.String("s", "", `Send signal to the daemon: stop - shutdown`)
)

const logFileName = "sample.log"
const pidFileName = "sample.pid"

var (
	stop = make(chan struct{})
	done = make(chan struct{})
)

type Spy struct {
	appName        string // Имя приложения
	appVersion     string // Версия приложения
	bootUniqueId   string // Уникальный ID загрузки хоста
	buildCpuArch   string // Архтитектура CPU для которой собиралась Qt
	currentCpuArch string // Архитектура CPU хоста
	kernelType     string // Тип ядра ОС
	kernelVersion  string // Версия ядра ОС
	hostName       string // Имя хоста
	hostUniqueId   string // Уникальный ID хоста
	productName    string // Название и версия ОС
}

type Action struct {
	hostUniqueId string // Уникальный ID хоста
	action       string // Акция
}

func main() {
	flag.Parse()
	daemon.AddCommand(daemon.StringFlag(signal, "stop"), syscall.SIGTERM, termHandler)
	log.Println("flag")
	daemonize()
}

func daemonize() {
	context := &daemon.Context{
		PidFileName: pidFileName,
		PidFilePerm: 0644,
		LogFileName: logFileName,
		LogFilePerm: 0640,
		WorkDir:     "./",
		Args:        []string{"[go-daemon sample]"},
		Umask:       027,
	}

	if len(daemon.ActiveFlags()) > 0 {
		d, err := context.Search()
		if err != nil {
			log.Fatalf("Unable send signal to the daemon: %s", err.Error())
		}

		daemon.SendCommands(d)
		return
	}

	d, err := context.Reborn()
	if err != nil {
		log.Fatalln(err)
	}
	if d != nil {
		return
	}
	defer context.Release()

	log.Println("----------")
	log.Println("daemon started")

	setupLog()

	go startServer()

	err = daemon.ServeSignals()
	if err != nil {
		log.Printf("Error: %s", err.Error())
	}
	log.Println("daemon terminated")
}

func setupLog() {
	lf, err := NewLogFile(logFileName, os.Stderr)
	if err != nil {
		log.Fatalf("Unable to create log file: %s", err.Error())
	}

	log.SetOutput(lf)

	rotateLogSignal := time.Tick(30 * time.Second)
	go func() {
		for {
			<-rotateLogSignal
			if err := lf.Rotate(); err != nil {
				log.Fatalf("Unable to rotate log: %s", err.Error())
			}
		}
	}()
}

func startServer() {
	db, err := connectToDatabase()
	if err != nil {
		log.Fatalln(err)
	}

	log.Println("database is opened", time.Now().Unix())

	listener, err := net.Listen("tcp", "2323")

	if err != nil {
		log.Fatalln(err)
	}
	defer func(listener net.Listener) {
		err := listener.Close()
		if err != nil {
			log.Fatal(err)
		}
	}(listener)
	log.Println("----------")
	log.Println("server is listening")

	for {
		select {
		case <-stop:
			break
		}
		conn, err := listener.Accept()
		if err != nil {
			break
		}
		go handleConnection(conn, db)
	}
	done <- struct{}{}
}

func connectToDatabase() (*sql.DB, error) {
	postgresqlConnection := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable", host, port, user, password, dbname)
	db, err := sql.Open("postgres", postgresqlConnection)
	if err != nil {
		return nil, err
	}
	defer func(db *sql.DB) {
		err := db.Close()
		if err != nil {
			log.Fatalln(err)
		}
	}(db)
	return db, nil
}

func handleConnection(conn net.Conn, db *sql.DB) {

	log.Println("client connected: ", conn.RemoteAddr().String(), conn.LocalAddr().String())

	var (
		buffer = make([]byte, 1024)
		spy    Spy
	)

	for {
		length, err := conn.Read(buffer)
		if err != nil {
			log.Fatalln(err)
		}

		if length == 0 {
			continue
		} else {
			err = json.Unmarshal(buffer[:length], &spy)
			if err != nil {
				log.Fatalln(err)
			}
			err = saveToDatabase(db, spy)
			if err != nil {
				log.Fatalln(err)
			}
			break
		}
	}

	action, err := selectAction(db, spy.hostUniqueId)
	if err != nil {
		log.Fatalln(err)
	}

	b, err := json.Marshal(action)
	if err != nil {
		log.Fatalln(err)
	}
	conn.Write(b)
	conn.Close()
}

func saveToDatabase(db *sql.DB, spy Spy) error {
	_, err := db.Exec(`insert into "Spy" (
                      "app_name", 
                      "app_version", 
                      "boot_unique_id", 
                      "build_cpu_arch", 
                      "current_cpu_arch",
          			  "kernel_type", 
                      "kernel_version", 
                      "host_name", 
                      "host_unique_id", 
                      "product_name") values($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		spy.appName, spy.appVersion, spy.bootUniqueId, spy.buildCpuArch, spy.currentCpuArch, spy.kernelType,
		spy.kernelVersion, spy.hostName, spy.hostUniqueId, spy.productName)
	if err != nil {
		return err
	}
	return nil
}

func selectAction(db *sql.DB, hostUniqueId string) (Action, error) {
	action := Action{}
	row, err := db.Query(`select action from "Actions" where host_unique_id = $1`, hostUniqueId)
	if err != nil {
		return action, err
	}
	defer row.Close()
	row.Next()

	err = row.Scan(&action.action)
	if err != nil {
		return action, err
	}
	action.hostUniqueId = hostUniqueId
	return action, nil
}

func termHandler(sig os.Signal) error {
	log.Println("terminating...")
	stop <- struct{}{}
	if sig == syscall.SIGQUIT {
		<-done
	}
	return daemon.ErrStop
}
