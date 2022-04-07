package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/spf13/afero"
)

const TAGLENGTH = 14

type AksNodeCAWatcher struct {
	copyTimestamp string
	sourceDir     string
	destDir       string
	podFs         afero.Fs
}

func main() {
	nodeWatcher := &AksNodeCAWatcher{
		sourceDir: os.Getenv("TRUSTEDCASRCDIR"),
		destDir:   os.Getenv("TRUSTEDCADESTDIR"),
		podFs:     afero.NewOsFs(),
	}
	done := make(chan bool)
	runNodeWatcher(nodeWatcher, done)
	<-done
}

func runNodeWatcher(watcher *AksNodeCAWatcher, done chan bool) {
	ticker := time.NewTicker(time.Second * 5)
	go func() {
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				if !watcher.shouldIterationRun() {
					continue
				}
				watcher.copyTimestamp = keepOnlyNumbersInTag(time.Now().UTC().Format(time.RFC3339))
				err := watcher.runIteration()
				if err != nil {
					log.Println(err.Error())
				}
			}
		}
	}()
}

func (watcher *AksNodeCAWatcher) shouldIterationRun() bool {
	// TODO - how to handle case when we are unable to read src/dest dirs?
	srcCertFiles, wereFilesRead := tryReadingCertFiles(watcher.podFs, watcher.sourceDir)
	if !wereFilesRead {
		return false
	}
	destCertFiles, wereFilesRead := tryReadingCertFiles(watcher.podFs, watcher.destDir)
	if !wereFilesRead {
		return false
	}
	if wasSrcUpdated(srcCertFiles, destCertFiles) {
		return true
	}
	return doExistingFilesDiffer(watcher, srcCertFiles)
}

func tryReadingCertFiles(fs afero.Fs, dir string) ([]os.FileInfo, bool) {
	destCertFiles, err := getCertFilesFromDir(fs, dir)
	if err != nil {
		log.Printf("Couldn't get files from %s, error: %s.", dir, err.Error())
		return nil, false
	}
	return destCertFiles, true
}

func getCertFilesFromDir(fs afero.Fs, dir string) ([]os.FileInfo, error) {
	files, err := afero.ReadDir(fs, dir)
	if err != nil {
		return []os.FileInfo{}, err
	}
	return removeNonCertFiles(fs, dir, files), nil
}

func removeNonCertFiles(fs afero.Fs, dir string, files []os.FileInfo) []os.FileInfo {
	certFiles := make([]os.FileInfo, 0)
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		_, err := afero.ReadFile(fs, getFilePath(dir, file.Name()))
		if err != nil {
			continue
		}
		certFiles = append(certFiles, file)
	}
	return certFiles
}

func wasSrcUpdated(srcCertFiles, destCertFiles []os.FileInfo) bool {
	return len(srcCertFiles) != len(destCertFiles)
}

func doExistingFilesDiffer(watcher *AksNodeCAWatcher, srcFiles []os.FileInfo) bool {
	for _, srcFile := range srcFiles {
		eqDestFileContent := getEquivalentDestFileContent(watcher, srcFile.Name())
		// TODO - ReadFile err shouldn't happen here as we filter files above. But maybe it should be handled anyways?
		srcFileContent, _ := afero.ReadFile(watcher.podFs, getFilePath(watcher.sourceDir, srcFile.Name()))
		if !bytes.Equal(srcFileContent, eqDestFileContent) {
			return true
		}
	}
	return false
}

func getEquivalentDestFileContent(watcher *AksNodeCAWatcher, fileName string) []byte {
	matches, _ := afero.Glob(watcher.podFs, watcher.destDir+"/"+"*"+fileName)
	if len(matches) > 1 {
		log.Printf("Expected 1 match for %s, but found %d. Overwriting dest dir.", fileName, len(matches))
		return nil
	}
	if len(matches) == 0 {
		return nil
	}
	content, err := afero.ReadFile(watcher.podFs, matches[0])
	if err != nil {
		log.Printf("Couldn't read file %s, error: %s. Overwriting dest dir.", matches[0], err.Error())
		return nil
	}
	return content
}

func keepOnlyNumbersInTag(tag string) string {
	return strings.Map(
		func(r rune) rune {
			if unicode.IsDigit(r) {
				return r
			}
			return -1
		},
		tag,
	)
}

func (watcher *AksNodeCAWatcher) runIteration() error {
	err := watcher.removeOldFiles()
	if err != nil {
		return err
	}
	return watcher.moveFiles()
}

func (watcher *AksNodeCAWatcher) removeOldFiles() error {
	oldFiles, err := afero.ReadDir(watcher.podFs, watcher.destDir)
	if err != nil {
		return err
	}
	for _, file := range oldFiles {
		if watcher.shouldFileBeRemoved(file.Name()) {
			err = watcher.podFs.Remove(getFilePath(watcher.destDir, file.Name()))
			if err != nil {
				log.Printf("Couldn't remove file %s, error: %s", file.Name(), err.Error())
				continue
			}
		}
	}
	return nil
}

func (watcher *AksNodeCAWatcher) shouldFileBeRemoved(fileName string) bool {
	fileTimestampTag := fileName[:TAGLENGTH]
	return watcher.copyTimestamp > fileTimestampTag
}

func (watcher *AksNodeCAWatcher) moveFiles() error {
	files, err := afero.ReadDir(watcher.podFs, watcher.sourceDir)
	if err != nil {
		return err
	}
	for _, file := range files {
		fileContent, err := afero.ReadFile(watcher.podFs, getFilePath(watcher.sourceDir, file.Name()))
		if err != nil {
			log.Printf("Couldn't read file %s, error: %s", file.Name(), err.Error())
			continue
		}
		err = watcher.createTaggedFileInDestination(file.Name(), fileContent)
		if err != nil {
			log.Printf("Couldn't move file %s to destination, error: %s", file.Name(), err.Error())
		}
	}
	return nil
}

func (watcher *AksNodeCAWatcher) createTaggedFileInDestination(fileName string, fileContent []byte) error {
	taggedFileName := createTaggedFileName(fileName, watcher.copyTimestamp)
	return afero.WriteFile(watcher.podFs, getFilePath(watcher.destDir, taggedFileName), fileContent, 0644)
}

func getFilePath(dir, fileName string) string {
	return fmt.Sprintf("%s/%s", dir, fileName)
}

func createTaggedFileName(fileName, tag string) string {
	return fmt.Sprintf("%s%s%s", tag, getFileNameWithoutExtension(fileName), filepath.Ext(fileName))
}

func getFileNameWithoutExtension(fileName string) string {
	return strings.TrimSuffix(filepath.Base(fileName), filepath.Ext(fileName))
}
