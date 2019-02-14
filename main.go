package main

import (
	"flag"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/fsnotify/fsnotify"
	"github.com/kr/pty"
)

var debug bool

func debugLog(args ...interface{}) {
	if debug {
		log.Println(args...)
	}
}

func killCmd(cmd *exec.Cmd) (int, error) {
	pid := cmd.Process.Pid
	err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	return pid, err
}

func runCmd(cmd string, update chan int, stop chan bool, done chan bool) error {
	log.Printf("running command %q\n", cmd)

	c := exec.Command("/bin/sh", "-c", cmd)
	ptmx, err := pty.Start(c)
	if err != nil {
		return err
	}

	defer func() { ptmx.Close() }()

	go func() { io.Copy(ptmx, os.Stdin) }()

	go func(c *exec.Cmd) {
		<-update
		debugLog("updating command")
		_, err := killCmd(c)
		if err != nil {
			panic(err)
		}
		go runCmd(cmd, update, stop, done)
	}(c)

	go func(c *exec.Cmd) {
		<-stop
		debugLog("stopping command")
		_, err := killCmd(c)
		if err != nil {
			panic(err)
		}
		done <- true
	}(c)

	_, err = io.Copy(os.Stdout, ptmx)
	return err
}

func watch(pattern string, updates chan int, stop chan bool, done chan bool) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				debugLog("event:", event)
				if event.Op&fsnotify.Write == fsnotify.Write {
					log.Println("modified file:", event.Name)
					updates <- 1
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				debugLog("error:", err)
			}
		}
	}()

	var paths []string
	filepath.Walk(pattern, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return nil
		}
		// ignore hidden dirs
		if len(path) > 1 &&
			strings.HasPrefix(filepath.Base(path), ".") &&
			!strings.HasPrefix(filepath.Base(path), "..") {
			return filepath.SkipDir
		}
		paths = append(paths, path)
		return nil
	})

	for _, path := range paths {
		log.Printf("watching %s", path)
		err = watcher.Add(path)
		if err != nil {
			log.Fatal(err)
		}
	}
	<-stop
	debugLog("stopping watch")
	done <- true
}

func main() {
	watchPattern := flag.String("w", ".", "the glob to watch")
	flag.BoolVar(&debug, "debug", false, "")
	flag.Parse()
	cmd := strings.Join(flag.Args(), " ")
	updates := make(chan int)
	stopWatch := make(chan bool)
	stopCmd := make(chan bool)
	cmdDone := make(chan bool)
	watchDone := make(chan bool)
	go watch(*watchPattern, updates, stopWatch, watchDone)
	go runCmd(cmd, updates, stopCmd, cmdDone)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	done := make(chan bool)
	go func() {
		for {
			sig := <-sigs
			stopWatch <- true
			stopCmd <- true
			log.Printf("stopping on %s signal...", sig)
			done <- true
		}
	}()
	<-cmdDone
	<-watchDone
	<-done
}
