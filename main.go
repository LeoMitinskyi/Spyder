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
	AppName        string // Имя приложения
	AppVersion     string // Версия приложения
	BootUniqueId   string // Уникальный ID загрузки хоста
	BuildCpuArch   string // Архтитектура CPU для которой собиралась Qt
	CurrentCpuArch string // Архитектура CPU хоста
	KernelType     string // Тип ядра ОС
	KernelVersion  string // Версия ядра ОС
	HostName       string // Имя хоста
	HostUniqueId   string // Уникальный ID хоста
	ProductName    string // Название и версия ОС
}

type Action struct {
	HostUniqueId string // Уникальный ID хоста
	Action       string // Акция
}

func main() {
	flag.Parse()
	daemon.AddCommand(daemon.StringFlag(signal, "stop"), syscall.SIGTERM, termHandler)
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

	// go startServer()
	startServer()
	
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

	/*rotateLogSignal := time.Tick(30 * time.Second)
	go func() {
		for {
			<-rotateLogSignal
			if err := lf.Rotate(); err != nil {
				log.Fatalf("Unable to rotate log: %s", err.Error())
			}
		}
	}()*/
}

func startServer() {
	db, err := connectToDatabase()
	if err != nil {
		log.Fatalln(err)
	}

	log.Println("database is opened")

	listener, err := net.Listen("tcp", "localhost:15253")

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
		/*select {
		case <-stop:
			break
		}*/
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
	/*defer func(db *sql.DB) {
		err := db.Close()
		if err != nil {
			log.Fatalln(err)
		}
	}(db)*/
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
				log.Println("unmarshalling error")
				log.Fatalln(err)
			}
			
			log.Println("info from client: ", spy)
			err = saveToDatabase(db, spy)
			if err != nil {
				log.Fatalln(err)
			}
			log.Println("info from client successfully saved to db")
			break
		}
	}

	action, err := selectAction(db, spy.HostUniqueId)
	if err != nil {
		log.Fatalln(err)
	}
	log.Println("selected action for client: ", action)
	
	b, err := json.Marshal(action)
	if err != nil {
		log.Println("marshalling error")
		log.Fatalln(err)
	}
	conn.Write(b)
	conn.Close()
}

func saveToDatabase(db *sql.DB, spy Spy) error {
	_, err := db.Exec(`insert into "spyder"."spy" (
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
		spy.AppName, spy.AppVersion, spy.BootUniqueId, spy.BuildCpuArch, spy.CurrentCpuArch, spy.KernelType,
		spy.KernelVersion, spy.HostName, spy.HostUniqueId, spy.ProductName)
	if err != nil {
		log.Println("error save to database")
		return err
	}
	return nil
}

func selectAction(db *sql.DB, hostUniqueId string) (Action, error) {
	action := Action{}
	row, err := db.Query(`select action from "spyder"."actions" where host_unique_id = $1`, hostUniqueId)
	if err != nil {
		log.Println("selection error")
		return action, err
	}
	defer row.Close()
	row.Next()

	err = row.Scan(&action.Action)
	if err != nil {
		log.Println("scanning action error")
		return action, err
	}
	action.HostUniqueId = hostUniqueId
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
