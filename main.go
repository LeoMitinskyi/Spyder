package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	_ "github.com/lib/pq"
	"github.com/sevlyar/go-daemon"
	"log"
	"net"
)

const (
	host     = "127.0.0.1"
	port     = 5432
	user     = ""
	password = ""
	dbname   = "tlc"
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
	log.Println("flag")
	daemonize()
}

func daemonize() {
	context := &daemon.Context{
		PidFileName: "",
		PidFilePerm: 0,
		LogFileName: "",
		LogFilePerm: 0,
		WorkDir:     "",
		Chroot:      "",
		Env:         nil,
		Args:        nil,
		Umask:       0,
	}

	d, err := context.Reborn()
	if err != nil {
		log.Fatal(err)
	}
	if d != nil {
		return
	}
	defer func(context *daemon.Context) {
		err := context.Release()
		if err != nil {
			log.Fatal(err)
		}
	}(context)

	log.Println("----------")
	log.Println("daemon started")

	err = startServer()
	if err != nil {
		log.Fatal(err)
	}
}

func startServer() error {
	db, err := connectToDatabase()
	if err != nil {
		return err
	}

	listener, err := net.Listen("tcp", "2323")

	if err != nil {
		return err
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
		conn, err := listener.Accept()
		if err != nil {
			return err
		}
		go handleConnection(conn, db)
	}
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
			log.Fatal(err)
		}
	}(db)
	return db, nil
}

func handleConnection(conn net.Conn, db *sql.DB) {
	var (
		buffer = make([]byte, 1024)
		spy    Spy
	)
close:
	for {
		for {
			length, err := conn.Read(buffer)
			if err != nil {
				break close
			}

			if length == 0 {
				continue
			} else {
				err = json.Unmarshal(buffer[:length], &spy)
				if err != nil {
					panic(err)
				}
				err = saveToDatabase(db, spy)
				if err != nil {
					panic(err)
				}
				break
			}
		}

		action, err := selectAction(db, spy.hostUniqueId)
		if err != nil {
			panic(err)
		}

		b, err := json.Marshal(action)
		if err != nil {
			panic(err)
		}
		conn.Write(b)
	}
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
