package main

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/afero"
)

func TestCopyOperation(t *testing.T) {
	testFiles := []string{"ca1.txt", "c2.txt", "ca3.txt"}
	sourceDir := "source"
	destDir := "dest"

	t.Run("filesFromSourceShouldAppearInDestAfterCopy", func(t *testing.T) {
		watcher := setUpWatcherForTest(sourceDir, destDir)
		createTestFilesInFs(watcher.podFs, watcher.sourceDir, testFiles)

		err := watcher.moveFiles()
		if err != nil {
			t.Fatal(err)
		}

		if areFilesInDirectory(watcher.podFs, watcher.destDir, testFiles) != true {
			t.Fail()
		}
	})
	t.Run("FilesShouldBeMarkedWithTimestampAfterCopyToDest", func(t *testing.T) {
		watcher := setUpWatcherForTest(sourceDir, destDir)
		createTestFilesInFs(watcher.podFs, watcher.sourceDir, testFiles)

		err := watcher.moveFiles()
		if err != nil {
			t.Fatal(err)
		}

		if areFilesInDirectoryTagged(watcher.podFs, watcher.copyTimestamp, destDir) != true {
			t.Fail()
		}
	})
	t.Run("FilesWithOlderTimestampShouldBeRemovedInDestAfterCopy", func(t *testing.T) {
		watcher := setUpWatcherForTest(sourceDir, destDir)
		createTestFilesInFs(watcher.podFs, watcher.sourceDir, testFiles)

		watcher.copyTimestamp = keepOnlyNumbersInTag(time.Now().Add(-5 * time.Minute).UTC().Format(time.RFC3339))
		createAlreadyTaggedFilesForRemoval(watcher, testFiles)

		err := watcher.runIteration()
		if err != nil {
			t.Fatal(err)
		}

		files, err := afero.ReadDir(watcher.podFs, watcher.destDir)
		if err != nil {
			t.Fatal(err)
		}
		if !areOlderFilesDeleted(files, watcher.copyTimestamp) {
			t.Fail()
		}
	})
	t.Run("FilesShouldBeDeletedFromDestWhenAllSourceFilesAreDeleted", func(t *testing.T) {
		watcher := setUpWatcherForTest(sourceDir, destDir)
		createTestFilesInFs(watcher.podFs, watcher.sourceDir, testFiles)

		err := watcher.runIteration()
		if err != nil {
			t.Fatal(err)
		}

		deleteFilesInDir(watcher.podFs, watcher.sourceDir)
		watcher.copyTimestamp = keepOnlyNumbersInTag(time.Now().Add(-5 * time.Minute).UTC().Format(time.RFC3339))

		err = watcher.runIteration()
		if err != nil {
			t.Fatal(err)
		}
		if !isDirEmpty(watcher.podFs, watcher.destDir) {
			t.Fail()
		}
	})
}

func setUpWatcherForTest(sourceDir string, destDir string) *AksNodeCAWatcher {
	watcher := &AksNodeCAWatcher{
		copyTimestamp: keepOnlyNumbersInTag(time.Now().UTC().Format(time.RFC3339)),
		sourceDir:     sourceDir,
		destDir:       destDir,
		podFs:         createFileSystemForTest(sourceDir, destDir),
	}
	return watcher
}

func createFileSystemForTest(pathsToCreate ...string) afero.Fs {
	testFs := afero.NewMemMapFs()
	for _, path := range pathsToCreate {
		testFs.MkdirAll(path, 0644)
	}
	return testFs
}

func createTestFilesInFs(fs afero.Fs, path string, filesToCreate []string) {
	for _, fileName := range filesToCreate {
		afero.WriteFile(fs, path+"/"+fileName, []byte(fileName), 0644)
	}
}

func createAlreadyTaggedFilesForRemoval(watcher *AksNodeCAWatcher, filesToCreate []string) {
	for _, fileName := range filesToCreate {
		watcher.createTaggedFileInDestination(fileName, []byte(fileName))
	}
}

func areFilesInDirectory(targetFs afero.Fs, targetDir string, fileList []string) bool {
	for _, fileName := range fileList {
		matches, err := afero.Glob(targetFs, targetDir+"/"+"*"+strings.TrimSuffix(filepath.Base(fileName),
			filepath.Ext(fileName))+"*")
		if err != nil {
			return false
		}
		return len(matches) > 0
	}
	return false
}

func areFilesInDirectoryTagged(targetFs afero.Fs, tag, targetDir string) bool {
	files, err := afero.ReadDir(targetFs, targetDir)
	if err != nil {
		log.Fatal(err)
	}
	for _, file := range files {
		if !strings.Contains(file.Name(), tag) {
			return false
		}
	}
	return true
}

func areOlderFilesDeleted(files []os.FileInfo, timestamp string) bool {
	for _, file := range files {
		fileTimestampTag := file.Name()[:TAGLENGTH]
		if fileTimestampTag < timestamp {
			return false
		}
	}
	return true
}

func deleteFilesInDir(targetFs afero.Fs, targetDir string) {
	files, _ := afero.ReadDir(targetFs, targetDir)
	for _, file := range files {
		_ = targetFs.Remove(getFilePath(targetDir, file.Name()))
	}
}

func isDirEmpty(targetFs afero.Fs, targetDir string) bool {
	files, err := afero.ReadDir(targetFs, targetDir)
	if err != nil {
		return false
	}
	return len(files) == 0
}
