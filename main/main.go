package main

import (
	"io/fs"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

type AksNodeCAWatcher struct {
	copyTimestamp    string
	sourceDir        string
	destDir          string
	sourceFileSystem fs.FS
	destFileSystem   fs.FS
}

func main() {
	watcher := &AksNodeCAWatcher{
		copyTimestamp:    strconv.FormatInt(time.Now().Unix(), 10),
		sourceDir:        os.Getenv("TRUSTEDCASRCDIR"),
		destDir:          os.Getenv("TRUSTEDCADESTDIR"),
		sourceFileSystem: os.DirFS("TRUSTEDCASRCDIR"),
		destFileSystem:   os.DirFS("TRUSTEDCADESTDIR"),
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
	fs.WalkDir(watcher.sourceFileSystem, watcher.sourceDir, func(path string, d fs.DirEntry, err error) error {
		if d.IsDir() {
			return nil
		}
		fileContent, err := fs.ReadFile(watcher.sourceFileSystem, watcher.sourceDir+"/"+d.Name())
		if err != nil {
			return err
		}
		// TODO how to create file in another fs?
		os.WriteFile()
		watcher.destFileSystem.Open()
		return err
	})
	//files, err := fs.ReadDir(watcher.sourceFileSystem, watcher.sourceDir)
	//if err != nil {
	//	log.Fatal(err)
	//}
	//for _, file := range files {
	//err = os.Rename(watcher.sourceDir+"/"+file.Name(), watcher.destDir+"/"+strings.TrimSuffix(filepath.Base(file.Name()),
	//	filepath.Ext(file.Name()))+"-"+watcher.copyTimestamp+filepath.Ext(file.Name()))
	//if err != nil {
	//	log.Fatal(err)
	//}
}

func (watcher *AksNodeCAWatcher) shouldFileBeRemoved(fileName string) bool {
	fileTimestampTag := fileName[strings.LastIndex(fileName, "-")+1 : strings.Index(fileName, ".")]
	return watcher.copyTimestamp > fileTimestampTag
}

func (watcher *AksNodeCAWatcher) removeOldFiles() {
	oldFiles, err := fs.ReadDir(watcher.destFileSystem, ".")
	if err != nil {
		log.Fatal(err)
	}
	for _, file := range oldFiles {
		if watcher.shouldFileBeRemoved(file.Name()) {
			err = os.Remove(watcher.destDir + "/" + file.Name())
			if err != nil {
				log.Fatal(err)
			}
		}
	}
}
