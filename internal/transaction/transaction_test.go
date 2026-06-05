package transaction

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	src := filepath.Join(dir, "link")

	// Create target
	err := os.WriteFile(target, []byte("content"), 0644)
	require.NoError(t, err)

	log := &TransactionLog{}
	err = log.Symlink(src, target)
	require.NoError(t, err)

	// Verify symlink
	linkTarget, err := os.Readlink(src)
	require.NoError(t, err)
	assert.Equal(t, target, linkTarget)

	log.Commit()
}

func TestSymlink_Rollback(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	src := filepath.Join(dir, "link")

	err := os.WriteFile(target, []byte("content"), 0644)
	require.NoError(t, err)

	log := &TransactionLog{}
	err = log.Symlink(src, target)
	require.NoError(t, err)

	// Rollback
	log.Rollback()

	// Symlink should be gone
	_, err = os.Lstat(src)
	assert.True(t, os.IsNotExist(err))
}

func TestBackup(t *testing.T) {
	dir := t.TempDir()
	original := filepath.Join(dir, "original")
	backupPath := filepath.Join(dir, "original.orig")

	err := os.WriteFile(original, []byte("content"), 0644)
	require.NoError(t, err)

	log := &TransactionLog{}
	err = log.Backup(original, backupPath)
	require.NoError(t, err)

	// Original should be moved
	_, err = os.Stat(original)
	assert.True(t, os.IsNotExist(err))

	// Backup should exist
	_, err = os.Stat(backupPath)
	assert.NoError(t, err)

	log.Commit()
}

func TestBackup_Rollback(t *testing.T) {
	dir := t.TempDir()
	original := filepath.Join(dir, "original")
	backupPath := filepath.Join(dir, "original.orig")

	err := os.WriteFile(original, []byte("content"), 0644)
	require.NoError(t, err)

	log := &TransactionLog{}
	err = log.Backup(original, backupPath)
	require.NoError(t, err)

	// Rollback
	log.Rollback()

	// Original should be restored
	_, err = os.Stat(original)
	assert.NoError(t, err)

	// Backup should be gone
	_, err = os.Stat(backupPath)
	assert.True(t, os.IsNotExist(err))
}

func TestMkdir(t *testing.T) {
	dir := t.TempDir()
	newDir := filepath.Join(dir, "new", "nested", "dir")

	log := &TransactionLog{}
	err := log.Mkdir(newDir)
	require.NoError(t, err)

	_, err = os.Stat(newDir)
	assert.NoError(t, err)
	assert.True(t, true) // directory exists

	log.Commit()
}

func TestUnlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	link := filepath.Join(dir, "link")

	err := os.WriteFile(target, []byte("content"), 0644)
	require.NoError(t, err)
	err = os.Symlink(target, link)
	require.NoError(t, err)

	log := &TransactionLog{}
	err = log.Unlink(link)
	require.NoError(t, err)

	// Symlink should be gone
	_, err = os.Lstat(link)
	assert.True(t, os.IsNotExist(err))

	log.Commit()
}

func TestUnlink_Rollback(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	link := filepath.Join(dir, "link")

	err := os.WriteFile(target, []byte("content"), 0644)
	require.NoError(t, err)
	err = os.Symlink(target, link)
	require.NoError(t, err)

	log := &TransactionLog{}
	err = log.Unlink(link)
	require.NoError(t, err)

	// Rollback
	log.Rollback()

	// Symlink should be restored
	linkTarget, err := os.Readlink(link)
	require.NoError(t, err)
	assert.Equal(t, target, linkTarget)
}

func TestMove(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "source")
	dest := filepath.Join(dir, "dest", "file")

	err := os.WriteFile(src, []byte("content"), 0644)
	require.NoError(t, err)

	log := &TransactionLog{}
	err = log.Move(src, dest)
	require.NoError(t, err)

	// Source should be gone
	_, err = os.Stat(src)
	assert.True(t, os.IsNotExist(err))

	// Dest should exist
	_, err = os.Stat(dest)
	assert.NoError(t, err)

	log.Commit()
}

func TestMove_Rollback(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "source")
	dest := filepath.Join(dir, "dest", "file")

	err := os.WriteFile(src, []byte("content"), 0644)
	require.NoError(t, err)

	log := &TransactionLog{}
	err = log.Move(src, dest)
	require.NoError(t, err)

	// Rollback
	log.Rollback()

	// Source should be restored
	_, err = os.Stat(src)
	assert.NoError(t, err)

	// Dest should be gone
	_, err = os.Stat(dest)
	assert.True(t, os.IsNotExist(err))
}

func TestCommittedNoRollback(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	link := filepath.Join(dir, "link")

	err := os.WriteFile(target, []byte("content"), 0644)
	require.NoError(t, err)

	log := &TransactionLog{}
	err = log.Symlink(link, target)
	require.NoError(t, err)

	log.Commit()
	log.Rollback() // Should not undo

	// Symlink should still exist
	_, err = os.Lstat(link)
	assert.NoError(t, err)
}

func TestTOCTOU_Safe(t *testing.T) {
	dir := t.TempDir()
	original := filepath.Join(dir, "original")
	backupPath := filepath.Join(dir, "original.orig")

	// Backup when file doesn't exist (simulates TOCTOU race)
	log := &TransactionLog{}
	err := log.Backup(original, backupPath)
	require.NoError(t, err) // Should not error when file doesn't exist

	_, err = os.Stat(backupPath)
	assert.True(t, os.IsNotExist(err))

	// Unlink when file doesn't exist
	err = log.Unlink(original) // Should not panic
	assert.NoError(t, err)
}
