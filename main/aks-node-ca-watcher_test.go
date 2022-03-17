package main

import (
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/spf13/afero"
)

func TestCopyOperation(t *testing.T) {
	testFiles := []string{"ca1.txt", "c2.txt", "ca3.txt"}
	testFilesForRemoval := []string{"ca1-1.txt", "ca2-2.txt", "ca3-3.txt"}
	sourceDir := "source"
	destDir := "dest"

	t.Run("filesFromSourceShouldAppearInDestAfterCopy", func(t *testing.T) {
		watcher := setUpWatcherForTest(sourceDir, destDir, testFiles, testFilesForRemoval)
		watcher.tagAndCopyFiles()
		if areFilesInDirectory(testFiles, destDir) != true {
			t.Fail()
		}
	})
	t.Run("FilesShouldBeMarkedWithTimestampAfterCopyToDest", func(t *testing.T) {
		watcher := setUpWatcherForTest(sourceDir, destDir, testFiles, testFilesForRemoval)
		watcher.tagAndCopyFiles()
		if areFilesInDirectoryTagged(destDir, watcher.copyTimestamp) != true {
			t.Fail()
		}
	})
	t.Run("FilesWithOlderTimestampShouldBeRemovedInDestAfterCopy", func(t *testing.T) {
		watcher := setUpWatcherForTest(sourceDir, destDir, testFiles, testFilesForRemoval)
		watcher.runIteration()
		files, err := fs.ReadDir(watcher.destFileSystem, watcher.destDir)
		if err != nil {
			t.Fail()
		}
		if !areOlderFilesDeleted(files, watcher.copyTimestamp) {
			t.Fail()
		}
	})
}

func setUpWatcherForTest(sourceDir string, destDir string, testFiles []string, testFilesForRemoval []string) *AksNodeCAWatcher {
	sourceFs, destFs := prepareFileEnvForTesting(sourceDir, destDir, testFiles, testFilesForRemoval)
	watcher := &AksNodeCAWatcher{
		copyTimestamp:    strconv.FormatInt(time.Now().Unix(), 10),
		sourceDir:        sourceDir,
		destDir:          destDir,
		sourceFileSystem: sourceFs,
		destFileSystem:   destFs,
	}
	return watcher
}

func prepareFileEnvForTesting(sourceDir, destDir string, filesAtSource, filesAtDest []string) (afero.IOFS, afero.IOFS) {
	sourceFs := prepareFS(sourceDir, filesAtSource)
	destFs := prepareFS(destDir, filesAtDest)
	return sourceFs, destFs
}

func prepareFS(path string, filesToCreate []string) afero.IOFS {
	fs := afero.NewIOFS(afero.NewMemMapFs())
	fs.MkdirAll(path, 0644)
	createTestFilesInFs(path, fs.Fs, filesToCreate)
	return fs
}

func createTestFilesInFs(path string, fs afero.Fs, filesToCreate []string) {
	for _, fileName := range filesToCreate {
		afero.WriteFile(fs, path+"/"+fileName, []byte(fileName), 0644)
	}
}

func areFilesInDirectory(fileList []string, targetDir string) bool {
	for _, fileName := range fileList {
		matches, err := filepath.Glob(targetDir + "/" + strings.TrimSuffix(filepath.Base(fileName),
			filepath.Ext(fileName)) + "*")
		if err != nil {
			return false
		}
		return len(matches) > 0
	}
	return false
}

func areFilesInDirectoryTagged(targetDir, tag string) bool {
	files, err := os.ReadDir(targetDir)
	if err != nil {
		log.Fatal(err)
		return false
	}
	for _, file := range files {
		if !strings.Contains(file.Name(), tag) {
			return false
		}
	}
	return true
}

func areOlderFilesDeleted(files []os.DirEntry, timestamp string) bool {
	for _, file := range files {
		fileTimestampTag := file.Name()[strings.LastIndex(file.Name(), "-")+1 : strings.Index(file.Name(), ".")]
		if fileTimestampTag < timestamp {
			return false
		}
	}
	return true
}
