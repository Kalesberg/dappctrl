package main

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"pxctrl/util"
	"pxctrl/vpn"
	"strconv"
)

type config struct {
	LogFile    string
	PxctrlAddr string
	PxctrlTLS  bool
	SessionDir string
}

func newConfig() *config {
	conf := vpn.NewConfig()
	return &config{
		LogFile:    "fxtrig.log",
		PxctrlAddr: conf.Addr,
		PxctrlTLS:  conf.TLS,
		SessionDir: "sessions",
	}
}

const (
	logPerm  = 0644
	sessPerm = 0644
)

func main() {
	conf := newConfig()
	name := util.ExeDirJoin("pxtrig.config.json")
	if err := util.ReadJSONFile(name, &conf); err != nil {
		log.Fatalf("failed to read configuration: %s\n", err)
	}

	if len(conf.LogFile) != 0 {
		out, err := os.OpenFile(conf.LogFile,
			os.O_RDWR|os.O_CREATE|os.O_APPEND, logPerm)
		if err != nil {
			log.Fatalf("error opening file: %v", err)
		}
		defer out.Close()

		log.SetOutput(out)
	}

	switch os.Getenv("script_type") {
	case "user-pass-verify":
		handleAuth(conf)
	case "client-connect":
		handleConnect(conf)
	case "client-disconnect":
		handleDisconnect(conf)
	default:
		log.Fatalf("unsupported script_type")
	}
}

func getCreds() (string, string) {
	user := os.Getenv("username")
	pass := os.Getenv("password")

	if len(user) != 0 && len(pass) != 0 {
		return user, pass
	}

	if len(os.Args) < 2 {
		log.Fatalf("no filename passed to read credentials")
	}

	file, err := os.Open(os.Args[1])
	if err != nil {
		log.Fatal("failed to open file with credentials: %s", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Scan()
	user = scanner.Text()
	scanner.Scan()
	pass = scanner.Text()

	if err := scanner.Err(); err != nil {
		log.Fatal("failed to read file with credentials: %s", err)
	}

	return user, pass
}

func post(conf *config, path string, req interface{}, rep interface{}) {
	data, err := json.Marshal(req)
	if err != nil {
		log.Fatalf("failed to encode request: %s", err)
	}

	var proto = "http"
	if conf.PxctrlTLS {
		proto += "s"
	}

	resp, err := http.Post(proto+"://"+conf.PxctrlAddr+path,
		"application/json", bytes.NewReader(data))
	if err != nil {
		log.Fatalf("failed to post request: %s", err)
	}
	defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(rep); err != nil {
		log.Fatalf("failed to decode reply: %s", err)
	}
}

func commonName() string {
	cn := os.Getenv("common_name")
	if len(cn) == 0 {
		log.Fatalf("empty common_name")
	}
	return base64.URLEncoding.EncodeToString([]byte(cn))
}

func storeSession(conf *config, session string) {
	name := filepath.Join(conf.SessionDir, commonName())
	err := ioutil.WriteFile(name, []byte(session), sessPerm)
	if err != nil {
		log.Fatalf("failed to store session: %s", err)
	}
}

func loadSession(conf *config) string {
	name := filepath.Join(conf.SessionDir, commonName())
	data, err := ioutil.ReadFile(name)
	if err != nil {
		log.Fatalf("failed to load session: %s", err)
	}
	return string(data)
}

func handleAuth(conf *config) {
	user, pass := getCreds()

	req := vpn.AuthRequest{PaymentID: user, PaymentPassword: pass}

	var rep vpn.AuthReply
	post(conf, vpn.PathAuthenticate, req, &rep)
	if len(rep.Error) != 0 {
		log.Fatalf("failed to authenticate %s: %s", user, rep.Error)
	}

	storeSession(conf, rep.SessionID)
}

func handleConnect(conf *config) {
	port, err := strconv.Atoi(os.Getenv("trusted_port"))
	if err != nil || port <= 0 || port > 0xFFFF {
		log.Fatalf("bad trusted_port value")
	}

	req := vpn.StartSessionRequest{
		SessionID:  loadSession(conf),
		PxctrlIP:   os.Getenv("ifconfig_remote"),
		ClientIP:   os.Getenv("trusted_ip"),
		ClientPort: uint16(port),
	}

	var rep vpn.StartSessionReply
	post(conf, vpn.PathStartSession, req, &rep)
	if len(rep.Error) != 0 {
		log.Fatalf("failed to start session: %s", rep.Error)
	}
}

func handleDisconnect(conf *config) {
	down, err := strconv.Atoi(os.Getenv("bytes_sent"))
	if err != nil || down < 0 {
		log.Fatalf("bad bytes_sent value")
	}

	up, err := strconv.Atoi(os.Getenv("bytes_received"))
	if err != nil || up < 0 {
		log.Fatalf("bad bytes_received value")
	}

	req := vpn.StopSessionRequest{
		SessionID: loadSession(conf),
		DownKiBs:  uint(down / 1024),
		UpKiBs:    uint(up / 1024),
	}

	var rep vpn.StopSessionReply
	post(conf, vpn.PathStopSession, req, &rep)
	if len(rep.Error) != 0 {
		log.Fatalf("failed to stop session: %s", rep.Error)
	}
}
