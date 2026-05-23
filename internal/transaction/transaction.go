package transaction

import (
	"os"
	"path/filepath"
)

// ActionType describes a filesystem operation.
type ActionType int

const (
	ActionSymlink ActionType = iota
	ActionBackup
	ActionMkdir
	ActionUnlink
	ActionMove
)

func (a ActionType) String() string {
	switch a {
	case ActionSymlink:
		return "symlink"
	case ActionBackup:
		return "backup"
	case ActionMkdir:
		return "mkdir"
	case ActionUnlink:
		return "unlink"
	case ActionMove:
		return "move"
	default:
		return "unknown"
	}
}

// LinkAction represents a single recorded filesystem operation.
type LinkAction struct {
	Type       ActionType
	Path       string
	Target     string
	BackupPath string
}

// TransactionLog records filesystem operations for potential rollback.
// Usage:
//
//	log := &TransactionLog{}
//	if err := log.Symlink(dest, src); err != nil {
//	    log.Rollback()
//	    return err
//	}
//	log.Commit()
type TransactionLog struct {
	actions   []LinkAction
	committed bool
}

// Symlink creates a symlink and records it.
func (t *TransactionLog) Symlink(path, target string) error {
	if err := os.Symlink(target, path); err != nil {
		return err
	}
	t.actions = append(t.actions, LinkAction{
		Type:   ActionSymlink,
		Path:   path,
		Target: target,
	})
	return nil
}

// Backup moves a file to backup and records it. Safe for TOCTOU.
func (t *TransactionLog) Backup(path, backupPath string) {
	// Handle TOCTOU: check if path still exists
	if _, err := os.Lstat(path); os.IsNotExist(err) {
		return // Already gone, nothing to do
	}

	if err := os.Rename(path, backupPath); err != nil {
		return // Best-effort
	}

	t.actions = append(t.actions, LinkAction{
		Type:       ActionBackup,
		Path:       path,
		BackupPath: backupPath,
	})
}

// Move moves src → dest. Rollback restores dest → src.
func (t *TransactionLog) Move(src, dest string) error {
	if _, err := os.Stat(src); os.IsNotExist(err) {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}

	if err := os.Rename(src, dest); err != nil {
		return err
	}

	t.actions = append(t.actions, LinkAction{
		Type:   ActionMove,
		Path:   src,
		Target: dest,
	})
	return nil
}

// Mkdir creates a directory and records it.
func (t *TransactionLog) Mkdir(path string) error {
	if err := os.MkdirAll(path, 0755); err != nil {
		return err
	}
	t.actions = append(t.actions, LinkAction{
		Type: ActionMkdir,
		Path: path,
	})
	return nil
}

// Unlink removes a symlink and records it. Safe for TOCTOU.
func (t *TransactionLog) Unlink(path string) error {
	// Check if symlink exists (broken or not)
	if _, err := os.Lstat(path); os.IsNotExist(err) {
		return nil // Already gone
	}

	// Readlink to get target for potential rollback
	target := ""
	if link, err := os.Readlink(path); err == nil {
		target = link
	}

	if err := os.Remove(path); err != nil {
		return err
	}

	t.actions = append(t.actions, LinkAction{
		Type:   ActionUnlink,
		Path:   path,
		Target: target,
	})
	return nil
}

// Commit marks the transaction as successful (no rollback needed).
func (t *TransactionLog) Commit() {
	t.committed = true
}

// Rollback undoes all recorded operations in reverse order.
func (t *TransactionLog) Rollback() {
	if t.committed {
		return
	}

	for i := len(t.actions) - 1; i >= 0; i-- {
		act := t.actions[i]
		switch act.Type {
		case ActionSymlink:
			// Remove the symlink we created
			if linkTarget, err := os.Readlink(act.Path); err == nil && linkTarget == act.Target {
				os.Remove(act.Path)
			}
		case ActionBackup:
			// Restore the backup
			if _, err := os.Stat(act.BackupPath); err == nil {
				os.Rename(act.BackupPath, act.Path)
			}
		case ActionMove:
			// Restore the move
			if _, err := os.Stat(act.Target); err == nil {
				os.Rename(act.Target, act.Path)
			}
		case ActionMkdir:
			// Remove directory if empty
			if fi, err := os.Stat(act.Path); err == nil && fi.IsDir() {
				os.Remove(act.Path) // Will only succeed if empty
			}
		case ActionUnlink:
			// Restore the symlink we removed
			if act.Target != "" {
				if _, err := os.Lstat(act.Path); os.IsNotExist(err) {
					os.Symlink(act.Target, act.Path)
				}
			}
		}
	}
}
