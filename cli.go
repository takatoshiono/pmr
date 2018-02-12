package main

import (
	"bufio"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/sirupsen/logrus"
)

// Exit codes are int values that represent an exit code for a particular error.
const (
	ExitCodeOK    int = 0
	ExitCodeError int = 1 + iota
)

// CLI is the command line object
type CLI struct {
	// outStream and errStream are the stdout and stderr
	// to write message from the CLI.
	outStream, errStream io.Writer
}

// Run invokes the CLI with the given arguments.
func (cli *CLI) Run(args []string) int {
	var (
		timeout  int
		url      string
		insecure bool

		version bool
	)

	// Define option flag parse
	flags := flag.NewFlagSet(Name, flag.ContinueOnError)
	flags.SetOutput(cli.errStream)

	flags.IntVar(&timeout, "timeout", 3, "request timeout sec")
	flags.IntVar(&timeout, "t", 3, "request timeout sec(Short)")
	flags.StringVar(&url, "url", "", "url")
	flags.StringVar(&url, "u", "", "url(Short)")
	flags.BoolVar(&insecure, "insecure", false, "Allow connections to SSL sites without certs")
	flags.BoolVar(&insecure, "k", false, "Allow connections to SSL sites without certs(Short)")

	flags.BoolVar(&version, "version", false, "Print version information and quit.")

	// Parse commandline flag
	if err := flags.Parse(args[1:]); err != nil {
		return ExitCodeError
	}

	// Show version
	if version {
		fmt.Fprintf(cli.errStream, "%s version %s\n", Name, Version)
		return ExitCodeOK
	}

	body, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		logrus.Fatal(err)
	}
	lines := strings.Split(string(body), "\n")

	eg := errgroup.Group{}
	for _, l := range lines {
		// スコープが変わるため代入が必要
		l := l
		eg.Go(func() error {
			return request(url, timeout, insecure, l)
		})
	}
	if err := eg.Wait(); err != nil {
		logrus.Fatal(err)
	}
	return ExitCodeOK
}

func request(url string, timeout int, insecure bool, filePath string) error {
	u, err := urlJoin(url, filePath)
	if err != nil {
		return err
	}
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: insecure},
	}
	client := &http.Client{
		Transport: tr,
		Timeout:   time.Duration(timeout) * time.Second,
	}
	r, err := client.Get(u)
	if err != nil {
		return err
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return err
	}

	logrus.Infof("request: %s %s", u, r.Status)
	fl, err := getFirstLine(filePath)
	if err != nil {
		return err
	}

	if strings.Index(string(body), fl) > 0 {
		logrus.Warnf("This file is published %s", filePath)
	}
	return nil
}

func urlJoin(base, path string) (string, error) {
	u, err := url.Parse(path)
	if err != nil {
		return "", err
	}
	pb, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	return pb.ResolveReference(u).String(), nil
}

func getFirstLine(path string) (string, error) {
	fp, err := os.Open(path)
	if err != nil {
		if err.Error() == "open : no such file or directory" {
			return "", nil
		}
		return "", err
	}

	defer fp.Close()
	reader := bufio.NewReaderSize(fp, 4096)
	for line := ""; err == nil; line, err = reader.ReadString('\n') {
		if len(line) > 5 {
			return line, err
		}
	}
	return "", nil
}
