package main

import (
	"flag"
	"log"

	"dappctrl/bcmon"
	"dappctrl/data"
	//"dappctrl/payment"
	//"dappctrl/somc"
	"dappctrl/util"
	//vpnmon "dappctrl/vpn/mon"
	//vpnsrv "dappctrl/vpn/srv"
	"os"
)

type config struct {
	DB  *data.DBConfig
	Log *util.LogConfig
	//PaymentServer     *payment.Config
	//SOMC              *somc.Config
	//VPNServer         *vpnsrv.Config
	//VPNMonitor        *vpnmon.Config
	BlockChainMonitor *bcmon.Config
}

func newConfig() *config {
	return &config{
		DB:  data.NewDBConfig(),
		Log: util.NewLogConfig(),
		//SOMC:              somc.NewConfig(),
		//VPNServer:         vpnsrv.NewConfig(),
		//VPNMonitor:        vpnmon.NewConfig(),
		BlockChainMonitor: bcmon.NewDefaultConfig(),
	}
}

func main() {
	fconfig := flag.String(
		"config", "dappctrl.config.json", "Configuration file")
	flag.Parse()

	conf := newConfig()
	if err := util.ReadJSONFile(*fconfig, &conf); err != nil {
		log.Fatalf("failed to read configuration: %s", err)
	}

	logger, err := util.NewLogger(conf.Log)
	if err != nil {
		log.Fatalf("failed to create logger: %s", err)
	}

	db, err := data.NewDB(conf.DB, logger)
	if err != nil {
		logger.Fatal("failed to open db connection: %s", err)
	}
	defer data.CloseDB(db)

	//srv := vpnsrv.NewServer(conf.VPNServer, logger, db)
	//defer srv.Close()
	//go func() {
	//	logger.Fatal("failed to serve vpn session requests: %s",
	//		srv.ListenAndServe())
	//}()
	//
	//mon := vpnmon.NewMonitor(conf.VPNMonitor, logger, db)
	//defer mon.Close()
	//go func() {
	//	logger.Fatal("failed to monitor vpn traffic: %s\n",
	//		mon.MonitorTraffic())
	//}()

	chainMonitor, err := bcmon.NewMonitor(conf.BlockChainMonitor, logger, db)
	if err != nil {
		log.Fatalf("failed to init blockchain monitor: %s", err)
	}
	defer chainMonitor.Close()
	go func() {
		logger.Fatal("failed to monitor blockchain events: %s\n",
			chainMonitor.MonitorEvents())
	}()

	// todo: remove me
	ch := make(chan bool)
	select {
	case <-ch:
		{
			os.Exit(1)
		}
	}

	//pmt := payment.NewServer(conf.PaymentServer, logger, db)
	//logger.Fatal("failed to start payment server: %s",
	//	pmt.ListenAndServe())
}
