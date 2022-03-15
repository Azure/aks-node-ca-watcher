package main

import (
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type AksNodeCAWatcher struct {
	copyTimestamp int64
	sourceDir     string
	destDir       string
}

func main() {
	watcher := &AksNodeCAWatcher{
		copyTimestamp: time.Now().Unix(),
		sourceDir:     os.Getenv("TRUSTEDCASRCDIR"),
		destDir:       os.Getenv("TRUSTEDCADESTDIR"),
	}
	for {
		watcher.runIteration()
		time.Sleep(time.Minute * 5)
	}
}

func (watcher *AksNodeCAWatcher) runIteration() {
	watcher.removeOldFiles()
	watcher.tagAndCopyFiles()
}

func (watcher *AksNodeCAWatcher) tagAndCopyFiles() {
	files, err := os.ReadDir(watcher.sourceDir)

	if err != nil {
		log.Fatal(err)
	}
	tag := strconv.FormatInt(watcher.copyTimestamp, 10)
	for _, file := range files {
		err = os.Rename(watcher.sourceDir+"/"+file.Name(), watcher.destDir+"/"+strings.TrimSuffix(filepath.Base(file.Name()),
			filepath.Ext(file.Name()))+"-"+tag+filepath.Ext(file.Name()))
		if err != nil {
			log.Fatal(err)
		}
	}
}

func (watcher *AksNodeCAWatcher) shouldBeRemoved(fileName string) bool {
	fileTimestampTag, err := strconv.ParseInt(fileName[strings.LastIndex(fileName,
		"-")+1:strings.Index(fileName, ".")], 10, 64)
	if err != nil {
		return false
	}
	return watcher.copyTimestamp > fileTimestampTag
}

func (watcher *AksNodeCAWatcher) removeOldFiles() {
	oldFiles, err := os.ReadDir(watcher.destDir)
	if err != nil {
		log.Fatal(err)
	}
	for _, file := range oldFiles {
		if watcher.shouldBeRemoved(file.Name()) {
			err = os.Remove(watcher.destDir + "/" + file.Name())
			if err != nil {
				log.Fatal(err)
			}
		}
	}
}
