// prune256.go         (c) 2014 David Rook
//
//  2014-02-26 started
//
//
// Note: TrimRight is necessary to prepare for re2 - but in rare cases it may
// trim chars off a pathelogical filename that really contains \t | \r | \n at the end
// This could be avoided by quoting the name but at the cost of rewriting ls256
// and all support programs that use *.256
package main

import (
	// go 1.2 stdlib pkgs
	"bufio"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	// local pkgs
	"github.com/hotei/mdr"
	"github.com/hotei/tokenbucket"
)

const (
	G_version = "prune.go   (c) 2014 David Rook version 0.0.2"
)

var (
	// Flags
	doReALLy bool
	killFile string
	//doKill bool
	countFile string
	//doCount bool
	listFile string
	//doList bool
	showZeroCt = false // could use a flag here if desired
	niceFlag   int

	tok           *tokenbucket.TokenBucket
	CantCreateRec = errors.New("prune: cant create rec")
	sigChan       chan os.Signal
	quitTime      bool

	g_nameList []mdr.Rec256
	g_argList  []string

	g_totalMatches   int64
	g_totalBytes     int64
	g_totalKillBytes int64
	g_totalKillCt    int64
	g_tmMutex        sync.Mutex
)

func usage() {
	u := `
Prune arguments (output of ls256) using re2 files

Typical usage:
	prune256 -count=count.re2 files.256 [*.256]
	
option flags:
	-nice          number in range [0..100] (100 ms increments) to delay between deletes
	-ReALLy		   remove duplicated files (as listed in trim.256)
	-kill=fname    file.re2 with kill targets
	-count=fname   file.re2 with count targets
	-list=fname    file.re2 with list targets	
	`
	fmt.Printf("%s\n", u)
	//	-version	print version number and quit
	//	-verbose	lots of info
	os.Exit(0)
}

func init() {
	sigChan = make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, os.Kill)
	log.SetFlags(log.Flags() | log.Lshortfile)

	flag.BoolVar(&doReALLy, "ReALLy", false, "actually delete")
	flag.StringVar(&killFile, "kill", "", "Kill file re2s")
	flag.StringVar(&countFile, "count", "", "Count file re2s")
	flag.StringVar(&listFile, "list", "", "List file re2s")
	flag.IntVar(&niceFlag, "nice", 0, "Be nice to other programs[0..100]")
}

// LoadSHA256Names returns the array of pathnames
func LoadSHA256Names(fname string) ([]mdr.Rec256, error) {
	Verbose.Printf("Loading from %s\n", fname)
	listOfNames := make([]mdr.Rec256, 0, 100000)
	input, err := os.Open(fname)
	if err != nil {
		log.Fatalf("can't open file %s\n", fname)
	}
	scanner := bufio.NewScanner(input)
	lineCt := 0
	fileCt := 0
	for scanner.Scan() {
		sline := scanner.Text()
		if err := scanner.Err(); err != nil {
			log.Fatalf("error reading from %s\n", fname)
		}
		lineCt++
		Verbose.Printf("line = %q\n", sline)
		if len(sline) < 3 { // must have at least 3 pipe symbols
			continue
		}
		if sline[0] == '#' {
			continue
		}
		r, err := mdr.Split256(string(sline))
		if err != nil {
			log.Fatalf("split failed\n")
		}
		r.SHA = ""
		r.Date = ""
		r.Name = strings.TrimRight(r.Name, "\n\r\t ") // trailing blank is intended
		listOfNames = append(listOfNames, r)
		fileCt++
	}
	fmt.Printf("added %d precalculated digests from %s\n", len(listOfNames), fname)
	return listOfNames, nil
}

// BUG(mdr): TODO: handleQuit() doesn't look right to os.Exit - but what is right thing to do?

func handleQuit() {
	for {
		select {
		case _ = <-sigChan:
			quitTime = true
			os.Exit(1)
		default:
			time.Sleep(1 * time.Second)
		}
	}
}

func getTargets(fname string) []string {
	input, err := os.Open(fname)
	if err != nil {
		log.Fatalf("can't open file %s\n", fname)
	}
	defer input.Close()
	targMap := make(map[string]byte)
	allTargets := []string{}
	scanner := bufio.NewScanner(input)
	//scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		sline := scanner.Text()
		if err := scanner.Err(); err != nil {
			log.Fatalf("error reading %s\n", fname)
		}
		//fmt.Printf("%q\n",sline)
		if len(sline) <= 0 {
			continue
		}
		if sline[0] == '#' { // comment line
			//fmt.Printf("%s \n",sline)
			continue
		}
		// strip whitespace left, if begins with // skip
		// find #, cut to that index if present
		// trim whitespace right
		// remove trailing comma if present
		_, exists := targMap[sline]
		if exists {
			continue
		}
		targMap[sline] = 0
		allTargets = append(allTargets, sline)
	}
	return allTargets
}

// modPool_CountMatches uses multi-cores (with throttle to nCPU) when > 2 cores available
// It does not list by default but will if Verbose
//  Uses worker pool
func modPool_CountMatches(fname string) {
	startTime := time.Now()
	fmt.Printf("Doing modPool_CountMatches()\n")
	var wg sync.WaitGroup
	allTargets := getTargets(fname)
	fmt.Printf("searching %d different targets\n", len(allTargets))
	nCPU := runtime.NumCPU()
	runtime.GOMAXPROCS(nCPU)
	workers := make(chan int, nCPU)
	fmt.Printf("using %d cores\n", nCPU)
	for _, t := range allTargets {
		if quitTime {
			fmt.Printf("Returning from quit signal\n")
			return
		}
		workers <- 1 // blocks if no workers available
		Verbose.Printf("goroutine starting with regexp %s\n", t)
		wg.Add(1)
		go func(target string) {
			defer wg.Done()
			re, err := regexp.Compile(target)
			if err != nil {
				fmt.Printf("!Err ---> can't compile regexp : %s \n", target)
				<-workers // free a worker core
				return
			}
			var matchCt = int64(0)
			var matchBytes = int64(0)
			for _, r := range g_nameList {
				if re.MatchString(r.Name) {
					matchCt++
					matchBytes += r.Size
				}
			}
			showMatchCounts(matchBytes, matchCt, target)
			g_tmMutex.Lock()
			g_totalMatches += matchCt
			g_totalBytes += matchBytes
			g_tmMutex.Unlock()
			<-workers // free a worker core
		}(t)
	}
	wg.Wait()
	elapsedTime := time.Now().Sub(startTime)
	//elapsedSeconds := elapsedTime.Seconds()
	fmt.Printf("# modPool run time is %s\n", mdr.HumanTime(elapsedTime))
}

// BUG(mdr): split version is much++ slower than expected - 10X more than pool
// and it's not obvious why...

// modPool_CountMatches uses multi-cores when available
// It does not list by default but will if Verbose
//  Uses mdr.JobSplit
func modSplit_CountMatches(fname string) {
	startTime := time.Now()
	fmt.Printf("Doing modSplit_CountMatches()\n")
	var wg sync.WaitGroup
	allTargets := getTargets(fname)
	nCPU := runtime.NumCPU()
	fmt.Printf("searching %d different targets with %d CPUs\n", len(allTargets), nCPU)
	runtime.GOMAXPROCS(nCPU)
	splits := mdr.JobSplit(len(g_nameList), nCPU)
	for _, target := range allTargets {
		if quitTime {
			fmt.Printf("Returning from quit signal\n")
			return
		}
		Verbose.Printf("goroutine starting with regexp %s\n", target)
		re, err := regexp.Compile(target)
		if err != nil {
			fmt.Printf("!Err ---> can't compile regexp : %s \n", target)
			continue
		}
		var matchCt = int64(0)
		var matchBytes = int64(0)
		for j := 0; j < nCPU; j++ {
			wg.Add(1)
			go func(lo, hi int)  {
				defer wg.Done()
				var fileCt  = int64(0)
				var byteCt = int64(0)
				for i := lo; i <= hi; i++ {
					r := g_nameList[i]
					if re.MatchString(r.Name) {
						fileCt++
						byteCt += r.Size
					}
				}
				g_tmMutex.Lock()
				matchCt += fileCt
				matchBytes += byteCt
				g_tmMutex.Unlock()
			}(splits[j].X, splits[j].Y)
		}
		wg.Wait()
		showMatchCounts(matchBytes, matchCt, target)
		g_totalMatches += matchCt
		g_totalBytes += matchBytes
	}
	elapsedTime := time.Now().Sub(startTime)
	//elapsedSeconds := elapsedTime.Seconds()
	fmt.Printf("# modSplit run time is %s\n", mdr.HumanTime(elapsedTime))
}

func showMatchCounts(bites, fileCt int64, target string) {
	if showZeroCt {
		fmt.Printf("# %17s bytes in %6d matches for %s\n", mdr.CommaFmtInt64(bites), fileCt, target)
	} else {
		if fileCt > 0 {
			fmt.Printf("# %17s bytes in %6d matches for %s\n", mdr.CommaFmtInt64(bites), fileCt, target)
		}
	}
}

// modGort_CountMatches similar to pool but doesn't try to limit the
// number of active goroutines.
//
func modGort_CountMatches(fname string) {
	startTime := time.Now()
	fmt.Printf("Doing modGort_CountMatches()\n")
	var wg sync.WaitGroup
	allTargets := getTargets(fname)
	nCPU := runtime.NumCPU()
	fmt.Printf("search ing %d different targets with %d CPUs\n", len(allTargets), nCPU)
	// test passes, throttles down when nCPU >= 2
	runtime.GOMAXPROCS(nCPU)
	for _, t := range allTargets {
		if quitTime {
			fmt.Printf("Returning from quit signal\n")
			return
		}
		Verbose.Printf("goroutine starting with regexp %s\n", t)
		wg.Add(1)
		go func(target string) {
			defer wg.Done()
			re, err := regexp.Compile(target)
			if err != nil {
				log.Fatalf("can't compile regexp - crashed here\n")
			}
			var matchCt = int64(0)
			var matchBytes = int64(0)
			for _, r := range g_nameList {
				if re.MatchString(r.Name) {
					matchCt++
					matchBytes += r.Size
				}
			}
			showMatchCounts(matchBytes, matchCt, target)
			g_tmMutex.Lock()
			g_totalMatches += matchCt
			g_totalBytes += matchBytes
			g_tmMutex.Unlock()
		}(t)
	}
	wg.Wait()
	elapsedTime := time.Now().Sub(startTime)
	//elapsedSeconds := elapsedTime.Seconds()
	fmt.Printf("# modGort run time is %s\n", mdr.HumanTime(elapsedTime))
}

// mod_KillMatches will delete files that match
// single thread so no mutex protection of global vars
func mod_KillMatches(fname string) {
	startTime := time.Now()
	fmt.Printf("Doing KillMatches()\n")
	allTargets := getTargets(fname)
	fmt.Printf("searching %d different targets\n", len(allTargets))
	for _, target := range allTargets {
		re, err := regexp.Compile(target)
		if err != nil {
			fmt.Printf("!Err ---> can't compile regexp : %s \n", target)
			continue
		}
		var matchCt = int64(0)
		var killCt = int64(0)
		var matchBytes = int64(0)
		var killBytes = int64(0)
		for _, r := range g_nameList {
			if re.MatchString(r.Name) {
				matchCt++
				matchBytes += r.Size
				Verbose.Printf("\tkilling %s\n", r.Name)
				if niceFlag > 0 {
					time.Sleep(tok.Take(int64(niceFlag))) // rate limiter if used
				}
				if doReALLy {
					err := os.Remove(r.Name)
					if err != nil {
						if os.IsNotExist(err) == false {
							log.Fatalf("Can't remove %s\n", r.Name)
						}
					}
					killCt++
					killBytes += r.Size
					g_totalKillCt++
					g_totalKillBytes += r.Size
				} else {
					fmt.Printf("need flag -doReALLy to actually remove %s\n", r.Name)
				}
				matchCt++
			}
		}
		showMatchCounts(matchBytes, matchCt, target)
		fmt.Printf("Kill %s had %d matches, %d files actually deleted with %s bytes\n",
			target, matchCt, killCt, mdr.CommaFmtInt64(killBytes))
	}
	elapsedTime := time.Now().Sub(startTime)
	//elapsedSeconds := elapsedTime.Seconds()
	fmt.Printf("# modKill run time is %s\n", mdr.HumanTime(elapsedTime))
}

// mod_ListMatches is single threaded
// It does list and maintains count as well
func mod_ListMatches(fname string) {
	startTime := time.Now()
	fmt.Printf("Doing ListMatches()\n")
	allTargets := getTargets(fname)
	fmt.Printf("searching %d different targets\n", len(allTargets))

	for _, target := range allTargets {
		matchCt := int64(0)
		matchBytes := int64(0)
		re, err := regexp.Compile(target)
		if err != nil {
			log.Fatalf("can't compile regexp - crashed here\n")
		}
		for _, r := range g_nameList {
			if re.MatchString(r.Name) {
				fmt.Printf("%16s bytes : %s\n", mdr.CommaFmtInt64(r.Size), r.Name)
				matchCt++
				matchBytes += r.Size
			}
		}
		showMatchCounts(matchBytes, matchCt, target)
		g_totalMatches += matchCt
		g_totalBytes += matchBytes
	}
	elapsedTime := time.Now().Sub(startTime)
	//elapsedSeconds := elapsedTime.Seconds()
	fmt.Printf("# modList run time is %s\n", mdr.HumanTime(elapsedTime))
}

func flagSetup() {
	// -nice=# flag
	if niceFlag < 0 {
		niceFlag = 0
	}
	if niceFlag > 100 {
		niceFlag = 100
	}
}

func main() {
	startTime := time.Now()
	flag.Parse()

	for i := 0; i < flag.NArg(); i++ {
		g_argList = append(g_argList, flag.Arg(i))
	}
	flagSetup()
	tok = tokenbucket.New(time.Millisecond*100, 20)
	Verbose.Printf("arglist %v\n", g_argList)
	if len(g_argList) < 1 {
		usage()
	}
	go handleQuit()

	for _, arg := range g_argList {
		if quitTime {
			fmt.Printf("Returning from quit signal\n")
			break
		}
		fmt.Printf("loading %s\n", arg)
		var err error
		g_nameList, err = LoadSHA256Names(arg)
		if err != nil {
			log.Fatalf("Crashed loading %s\n", arg)
		}
		/*
		if len(countFile) > 0 {
			modGort_CountMatches(countFile)
		}
		if len(countFile) > 0 {
			modSplit_CountMatches(countFile)
		}
		*/
		if len(countFile) > 0 {
			modPool_CountMatches(countFile)
		}
		if len(killFile) > 0 {
			fmt.Printf("calling mod_KillMatches\n")
			mod_KillMatches(killFile)
		}
		if len(listFile) > 0 {
			mod_ListMatches(listFile)
		}
	}
	fmt.Printf("# Final summary for all args:\n")
	fmt.Printf("# Total bytes matched = %s in %d files\n",
		mdr.CommaFmtInt64(g_totalBytes), g_totalMatches)
	fmt.Printf("# Deleted %s total bytes in %s files\n",
		mdr.CommaFmtInt64(g_totalKillBytes), mdr.CommaFmtInt64(g_totalKillCt))
	elapsedTime := time.Now().Sub(startTime)
	//elapsedSeconds := elapsedTime.Seconds()
	fmt.Printf("# Elapsed run time is %s\n", mdr.HumanTime(elapsedTime))
}
