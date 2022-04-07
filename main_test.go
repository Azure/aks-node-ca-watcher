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

func TestShouldIterationRun(t *testing.T) {
	testFiles := []string{"ca1.txt", "c2.txt", "ca3.txt"}
	sourceDir := "source"
	destDir := "dest"

	t.Run("shouldReturnTrueIfSrcDirIsNotEmptyAndDestDirIsEmpty", func(t *testing.T) {
		nodeWatcher := setUpNodeWatcherForTest(sourceDir, destDir)
		createTestFilesInFs(nodeWatcher.podFs, nodeWatcher.sourceDir, testFiles)

		if !nodeWatcher.shouldIterationRun() {
			t.Fail()
		}
	})
	t.Run("shouldReturnTrueIfSrcDirIsEmptyAndDestDirIsNotEmpty", func(t *testing.T) {
		nodeWatcher := setUpNodeWatcherForTest(sourceDir, destDir)
		createTestFilesInFs(nodeWatcher.podFs, nodeWatcher.sourceDir, testFiles)

		err := nodeWatcher.moveFiles()
		if err != nil {
			t.Fatal(err)
		}

		deleteFilesInDir(nodeWatcher.podFs, sourceDir)

		if !nodeWatcher.shouldIterationRun() {
			t.Fail()
		}
	})
	t.Run("shouldReturnFalseIfSrcDirHasExactSameFilesAsDestDir", func(t *testing.T) {
		nodeWatcher := setUpNodeWatcherForTest(sourceDir, destDir)
		createTestFilesInFs(nodeWatcher.podFs, nodeWatcher.sourceDir, testFiles)

		err := nodeWatcher.moveFiles()
		if err != nil {
			t.Fatal(err)
		}

		if nodeWatcher.shouldIterationRun() {
			t.Fail()
		}
	})
	t.Run("shouldReturnTrueIfSrcDirHasDifferentFilesThanDestDir", func(t *testing.T) {
		nodeWatcher := setUpNodeWatcherForTest(sourceDir, destDir)
		createTestFilesInFs(nodeWatcher.podFs, nodeWatcher.sourceDir, testFiles)

		err := nodeWatcher.moveFiles()
		if err != nil {
			t.Fatal(err)
		}

		deleteFilesInDir(nodeWatcher.podFs, sourceDir)
		newFilesForSource := []string{"ca1-modified.txt", "c2-modified.txt", "ca3-modified.txt"}
		createTestFilesInFs(nodeWatcher.podFs, nodeWatcher.sourceDir, newFilesForSource)

		if !nodeWatcher.shouldIterationRun() {
			t.Fail()
		}
	})
	t.Run("shouldReturnTrueIfContentOfFileInSrcDirChanged", func(t *testing.T) {
		nodeWatcher := setUpNodeWatcherForTest(sourceDir, destDir)
		createTestFilesInFs(nodeWatcher.podFs, nodeWatcher.sourceDir, testFiles)

		err := nodeWatcher.moveFiles()
		if err != nil {
			t.Fatal(err)
		}
		modifyFileContent(nodeWatcher.podFs, nodeWatcher.sourceDir, testFiles[0])

		if !nodeWatcher.shouldIterationRun() {
			t.Fail()
		}
	})
}

func modifyFileContent(fs afero.Fs, dir string, fileName string) {
	afero.WriteFile(fs, dir+"/"+fileName, []byte("new modified content"), 0644)
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
