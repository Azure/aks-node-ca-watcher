package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/afero"
)

type AksNodeCAWatcher struct {
	copyTimestamp string
	sourceDir     string
	destDir       string
	podFs         afero.Fs
}

func main() {
	watcher := &AksNodeCAWatcher{
		copyTimestamp: strconv.FormatInt(time.Now().Unix(), 10),
		sourceDir:     os.Getenv("TRUSTEDCASRCDIR"),
		destDir:       os.Getenv("TRUSTEDCADESTDIR"),
		podFs:         afero.NewOsFs(),
	}

	for {
		err := watcher.runIteration()
		if err != nil {
			log.Fatal(err)
		}
		time.Sleep(time.Minute * 5)
	}
}

func (watcher *AksNodeCAWatcher) runIteration() error {
	err := watcher.removeOldFiles()
	if err != nil {
		return err
	}
	return watcher.moveFiles()
}

func (watcher AksNodeCAWatcher) removeOldFiles() error {
	oldFiles, err := afero.ReadDir(watcher.podFs, watcher.destDir)
	if err != nil {
		return err
	}
	for _, file := range oldFiles {
		if watcher.shouldFileBeRemoved(file.Name()) {
			err = watcher.podFs.Remove(getFilePath(watcher.destDir, file.Name()))
			if err != nil {
				fmt.Printf("Couldn't remove file %s, error: %s", file.Name(), err.Error())
				continue
			}
		}
	}
	return nil
}

func (watcher AksNodeCAWatcher) shouldFileBeRemoved(fileName string) bool {
	fileTimestampTag := fileName[strings.LastIndex(fileName, "-")+1 : strings.Index(fileName, ".")]
	return watcher.copyTimestamp > fileTimestampTag
}

func (watcher AksNodeCAWatcher) moveFiles() error {
	files, err := afero.ReadDir(watcher.podFs, watcher.sourceDir)
	if err != nil {
		return err
	}
	for _, file := range files {
		fileContent, err := afero.ReadFile(watcher.podFs, getFilePath(watcher.sourceDir, file.Name()))
		if err != nil {
			fmt.Printf("Couldn't read file %s, error: %s", file.Name(), err.Error())
			continue
		}
		err = watcher.createTaggedFileInDestination(file.Name(), fileContent)
		if err != nil {
			fmt.Printf("Couldn't move file %s to destination, error: %s", file.Name(), err.Error())
		}
	}
	return nil
}

func (watcher AksNodeCAWatcher) createTaggedFileInDestination(fileName string, fileContent []byte) error {
	taggedFileName := createTaggedFileName(fileName, watcher.copyTimestamp)
	return afero.WriteFile(watcher.podFs, getFilePath(watcher.destDir, taggedFileName), fileContent, 0644)
}

func getFilePath(dir, fileName string) string {
	return fmt.Sprintf("%s/%s", dir, fileName)
}

func createTaggedFileName(fileName, tag string) string {
	return fmt.Sprintf("%s-%s%s", getFileNameWithoutExtension(fileName), tag, filepath.Ext(fileName))
}

func getFileNameWithoutExtension(fileName string) string {
	return strings.TrimSuffix(filepath.Base(fileName), filepath.Ext(fileName))
}
