package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"CursorAgent/config"
	"CursorAgent/cursor"
	"CursorAgent/input"
	"CursorAgent/job"
	"CursorAgent/server"
	"CursorAgent/util"

	"github.com/robfig/cron/v3"
	"github.com/spf13/pflag"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	configPath := pflag.StringP("config", "c", "config/option.yaml", "Path to config YAML")
	runOnce := pflag.Bool("run-once", false, "Run each task once then exit (no cron)")
	pflag.Parse()

	opt, err := config.LoadOption(*configPath)
	if err != nil {
		log.Fatal(err)
	}
	closeLog, err := util.SetupLogOutput(opt.LogFile)
	if err != nil {
		log.Fatalf("log file: %v", err)
	}
	defer closeLog()

	client := cursor.NewClient(opt.AgentCommand)

	var pluginListener *server.PluginListener
	if opt.PluginListenerPort != 0 {
		pluginListener = server.NewPluginListener(opt.PluginListenerPort, opt)
		pluginListener.Start()
	}

	var cr *cron.Cron
	if !*runOnce {
		cr = cron.New(cron.WithSeconds())
	}
	for i := range opt.Tasks {
		task := &opt.Tasks[i]
		if task.Input.Type == "" {
			log.Printf("skip task %s: missing Input.Type", task.Name)
			continue
		}
		source, err := input.NewSource(task)
		if err != nil {
			log.Printf("skip task %s: %v", task.Name, err)
			continue
		}
		j := job.NewJob(task, source, client, opt, pluginListener)
		if task.CronTime != "" && cr != nil {
			_, err := cr.AddFunc(task.CronTime, func() {
				log.Printf("cron run task: %s", task.Name)
				j.Run()
			})
			if err != nil {
				log.Printf("cron add task %s: %v", task.Name, err)
				continue
			}
			log.Printf("scheduled task: %s (cron: %s)", task.Name, task.CronTime)
		} else {
			if *runOnce || task.CronTime == "" {
				log.Printf("run task once: %s", task.Name)
				j.Run()
			}
		}
	}
	if cr != nil {
		cr.Start()
	}

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	<-ch
	log.Printf("shutting down...")
	if cr != nil {
		<-cr.Stop().Done()
	}
	if pluginListener != nil {
		_ = pluginListener.Stop()
	}
}
