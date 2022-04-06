package main

import (
	"fmt"
	"github.com/fsnotify/fsnotify"
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
		nodeWatcher := setUpNodeWatcherForTest(sourceDir, destDir)
		createTestFilesInFs(nodeWatcher.podFs, nodeWatcher.sourceDir, testFiles)

		err := nodeWatcher.moveFiles()
		if err != nil {
			t.Fatal(err)
		}

		if areFilesInDirectory(nodeWatcher.podFs, nodeWatcher.destDir, testFiles) != true {
			t.Fail()
		}
	})
	t.Run("FilesShouldBeMarkedWithTimestampAfterCopyToDest", func(t *testing.T) {
		nodeWatcher := setUpNodeWatcherForTest(sourceDir, destDir)
		createTestFilesInFs(nodeWatcher.podFs, nodeWatcher.sourceDir, testFiles)

		err := nodeWatcher.moveFiles()
		if err != nil {
			t.Fatal(err)
		}

		if areFilesInDirectoryTagged(nodeWatcher.podFs, nodeWatcher.copyTimestamp, destDir) != true {
			t.Fail()
		}
	})
	t.Run("FilesWithOlderTimestampShouldBeRemovedInDestAfterCopy", func(t *testing.T) {
		nodeWatcher := setUpNodeWatcherForTest(sourceDir, destDir)
		createTestFilesInFs(nodeWatcher.podFs, nodeWatcher.sourceDir, testFiles)

		nodeWatcher.copyTimestamp = keepOnlyNumbersInTag(time.Now().Add(-5 * time.Minute).UTC().Format(time.RFC3339))
		createAlreadyTaggedFilesForRemoval(nodeWatcher, testFiles)

		err := nodeWatcher.runIteration()
		if err != nil {
			t.Fatal(err)
		}

		files, err := afero.ReadDir(nodeWatcher.podFs, nodeWatcher.destDir)
		if err != nil {
			t.Fatal(err)
		}
		if !areOlderFilesDeleted(files, nodeWatcher.copyTimestamp) {
			t.Fail()
		}
	})
	t.Run("FilesShouldBeDeletedFromDestWhenAllSourceFilesAreDeleted", func(t *testing.T) {
		nodeWatcher := setUpNodeWatcherForTest(sourceDir, destDir)
		createTestFilesInFs(nodeWatcher.podFs, nodeWatcher.sourceDir, testFiles)

		err := nodeWatcher.runIteration()
		if err != nil {
			t.Fatal(err)
		}

		deleteFilesInDir(nodeWatcher.podFs, nodeWatcher.sourceDir)
		nodeWatcher.copyTimestamp = keepOnlyNumbersInTag(time.Now().Add(5 * time.Minute).UTC().Format(time.RFC3339))

		err = nodeWatcher.runIteration()
		if err != nil {
			t.Fatal(err)
		}
		if !isDirEmpty(nodeWatcher.podFs, nodeWatcher.destDir) {
			t.Fail()
		}
	})
}

type mockNodeWatcher struct {
	called bool
}

func (n *mockNodeWatcher) runIteration() error {
	n.called = true
	return nil
}

func TestFileWatching(t *testing.T) {
	t.Run("programIterationShouldRunWhenFileIsAddedInSource", func(t *testing.T) {
		nodeWatcher := &mockNodeWatcher{}
		setUpDirsForWatchTests()
		fileWatcher := setUpFileWatcherForTest("source")
		exit := make(chan bool)
		runNodeWatcher(nodeWatcher, fileWatcher, exit)

		addFileForWatchTest("source")
		time.Sleep(time.Second * 5)
		exit <- true

		if !nodeWatcher.called {
			t.Fail()
		}
	})
	t.Run("programIterationShouldRunWhenFileIsDeletedInSource", func(t *testing.T) {
		nodeWatcher := &mockNodeWatcher{}
		setUpDirsForWatchTests()
		fileWatcher := setUpFileWatcherForTest("source")
		addFileForWatchTest("source")
		exit := make(chan bool)
		runNodeWatcher(nodeWatcher, fileWatcher, exit)

		removeFileForWatchTest("source/testFile")
		time.Sleep(time.Second * 5)
		exit <- true

		if !nodeWatcher.called {
			t.Fail()
		}
	})
	t.Run("programIterationShouldRunWhenFileIsModifiedInSource", func(t *testing.T) {
		nodeWatcher := &mockNodeWatcher{}
		setUpDirsForWatchTests()
		fileWatcher := setUpFileWatcherForTest("source")
		addFileForWatchTest("source")
		exit := make(chan bool)
		runNodeWatcher(nodeWatcher, fileWatcher, exit)

		modifyFileForWatchTest("source/testFile")
		time.Sleep(time.Second * 5)
		exit <- true

		if !nodeWatcher.called {
			t.Fail()
		}
	})
	t.Cleanup(func() {
		_ = os.RemoveAll("source")
	})
}

func removeFileForWatchTest(path string) {
	err := os.Remove(path)
	if err != nil {
		log.Fatal(err)
	}
}

func modifyFileForWatchTest(fileName string) {
	_ = os.WriteFile(fileName, []byte("new content"), 0644)
}

func addFileForWatchTest(targetDir string) {
	fileName := "testFile"
	err := os.WriteFile(fmt.Sprintf("%s/%s", targetDir, fileName), []byte("Test file content"), 0644)
	if err != nil {
		log.Fatal(err)
	}
}

func setUpDirsForWatchTests() {
	err := os.Mkdir("source", 0777)
	if err != nil {
		log.Fatal(err)
	}
}

func setUpNodeWatcherForTest(sourceDir string, destDir string) *AksNodeCAWatcher {
	nodeWatcher := &AksNodeCAWatcher{
		copyTimestamp: keepOnlyNumbersInTag(time.Now().UTC().Format(time.RFC3339)),
		sourceDir:     sourceDir,
		destDir:       destDir,
		podFs:         createFileSystemForTest(sourceDir, destDir),
	}
	return nodeWatcher
}

func setUpFileWatcherForTest(dirToWatch string) *fsnotify.Watcher {
	fileWatcher, _ := fsnotify.NewWatcher()
	err := fileWatcher.Add(dirToWatch)
	if err != nil {
		log.Fatal(err)
	}
	return fileWatcher
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
