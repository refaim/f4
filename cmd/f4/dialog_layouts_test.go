package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/unxed/f4/fusefs"
	"github.com/unxed/f4/vfs"
	"github.com/unxed/vtui"
)

func TestAllDialogs_LayoutValidation(t *testing.T) {
	t.Cleanup(swapFrameManager(t))
	skipIfNoRelevantChanges(t, "layouts",
		"lang/*.lng",
		"lang/*.txt",
		"file_ops.go",
		"dialog_button_layout.go",
		"*_dialog*.go",
		"*_ui*.go",
		"*_settings*.go",
		"actions*.go",
		"dialog_layouts_test.go",
		"go.mod",
	)
	vtui.SetDefaultPalette()

	// 1. Temporary redirect of the config paths to prevent writing/reading from the user's home directory.
	tmpDir := t.TempDir()

	oldGetConfig := getUserConfigIniPath
	oldConfig := AppConfig
	defer func() {
		getUserConfigIniPath = oldGetConfig
		AppConfig = oldConfig
	}()
	getUserConfigIniPath = func() string {
		return filepath.Join(tmpDir, "settings.ini")
	}

	// Actions that are destructive, async, or mutate global state without a dialog.
	skipActions := map[string]bool{
		"app.quit":                         true,
		"panel.systemexplorer":             true,
		"app.togglewindowsize":             true,
		"panel.rescan":                     true, // no dialog
		"panel.swap":                       true, // no dialog
		"panel.toggle":                     true, // no dialog
		"panel.toggleleftpanel":            true, // no dialog
		"panel.togglerightpanel":           true, // no dialog
		"panel.togglepassivepanel":         true, // no dialog
		"panel.splitleft":                  true, // no dialog
		"panel.splitright":                 true, // no dialog
		"panel.splitup":                    true, // no dialog
		"panel.splitdown":                  true, // no dialog
		"panel.splitactiveup":              true, // no dialog
		"panel.splitactivedown":            true, // no dialog
		"panel.splitreset":                 true, // no dialog
		"panel.viewbrief":                  true, // no dialog
		"panel.viewmedium":                 true, // no dialog
		"panel.viewdetailed":               true, // no dialog
		"panel.viewwide":                   true, // no dialog
		"panel.sortbyname":                 true, // no dialog
		"panel.sortbyext":                  true, // no dialog
		"panel.sortbytime":                 true,
		"panel.sortbysize":                 true,
		"panel.sortunsorted":               true,
		"panel.togglekeybar":               true,
		"panel.toggleinfobytes":            true,
		"panel.togglehidden":               true,
		"panel.historyback":                true,
		"panel.historyforward":             true,
		"file.view":                        true, // async launch
		"file.edit":                        true, // async launch
		"file.new":                         true, // async launch
		"file.attributes":                  true, // async launch
		"file.findduplicates":              true, // async launch
		"terminal.viewlog":                 true, // async launch
		"terminal.editlog":                 true, // async launch
		"editor.switchtoviewer":            true, // async launch
		"viewer.switchtoeditor":            true, // async launch
		"editor.codepagenext":              true, // modifies config
		"viewer.codepagenext":              true, // modifies config
		"editor.save":                      true, // no dialog
		"editor.undo":                      true, // no dialog
		"editor.redo":                      true, // no dialog
		"editor.copy":                      true, // no dialog
		"editor.cut":                       true, // no dialog
		"editor.paste":                     true, // no dialog
		"editor.selectall":                 true, // no dialog
		"editor.deleteline":                true, // no dialog
		"editor.toggleovertype":            true, // no dialog
		"editor.searchnext":                true, // no dialog
		"editor.wordwrap":                  true, // no dialog
		"editor.showwhitespaces":           true, // no dialog
		"editor.insertleftpanelpath":       true, // no dialog
		"editor.insertrightpanelpath":      true, // no dialog
		"editor.insertactivepanelfilename": true, // no dialog
		"editor.deletespacersforward":      true, // no dialog
		"viewer.wrapmode":                  true, // no dialog
		"viewer.hexmode":                   true, // no dialog
		"panel.copypath":                   true, // no dialog
		"panel.copyname":                   true, // no dialog
		"panel.copyselectednames":          true, // no dialog
		"panel.copyselectedpaths":          true, // no dialog
		"panel.copyselectedrealpaths":      true, // no dialog
		"panel.invertselection":            true, // no dialog
		"panel.restoreselection":           true, // no dialog
		"app.screengrab":                   true, // full screen raw frame
		"app.plugring":                     true, // async fetch
		"panel.leftdrivemenu":              true, // relies on active pty/panels
		"panel.rightdrivemenu":             true, // relies on active pty/panels
		"panel.enterdirectory":             true, // no dialog
		"panel.insertfilename":             true, // no dialog
		"panel.insertleftpath":             true, // no dialog
		"panel.insertrightpath":            true, // no dialog
		"debug.dummyoperation":             true, // async queue
		"macro.reload":                     true, // no dialog; owns an asynchronous toast
		"panel.infopanel":                  true, // no dialog
		"panel.quickview":                  true, // no dialog
	}

	// 3. Create a dummy file in the temp directory so file operations (Copy, Edit, etc.)
	// have a valid target and will naturally display their progress/confirmation dialogs.
	srcFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(srcFile, []byte("dummy content"), 0600); err != nil {
		t.Fatal(err)
	}

	oldHotkeys := GlobalHotkeysMgr
	oldMacro := MacroMgr
	defer func() {
		GlobalHotkeysMgr = oldHotkeys
		MacroMgr = oldMacro
	}()
	GlobalHotkeysMgr = NewHotkeyManager(filepath.Join(tmpDir, "hotkeys.ini"))
	MacroMgr = NewMacroManager(filepath.Join(tmpDir, "key_macros.ini"))

	// Load all language packs so the validator can assert layout against all translations dynamically
	packs := LoadAllLanguagePacks()
	if len(packs) == 0 {
		packs = []vtui.LanguagePack{{Name: "current"}}
	}

	oldMountTaskRunner := runPanelMountTask
	runPanelMountTask = func(pf *PanelsFrame, label string, readOnly bool, _ func(context.Context) (*fusefs.Mount, error)) {
		reportMount(pf, label, &fusefs.Mount{
			MountPoint: filepath.Join(tmpDir, "layout-validation-mount"),
			ReadOnly:   readOnly,
		}, nil, readOnly)
	}
	t.Cleanup(func() { runPanelMountTask = oldMountTaskRunner })

	// Complex-script widths are handled by vtui's grapheme-cell shaping.
	for _, act := range GetActions() {
		name := act.Name
		if skipActions[strings.ToLower(name)] {
			continue
		}

		t.Run(name, func(t *testing.T) {
			rules := vtui.DefaultLayoutRules
			rules.MaxWidth = 120 // Allow configurator/large dialogs to exceed default 78 columns
			baseStrings := vtui.SnapshotStrings()
			defer vtui.ReplaceStrings(baseStrings)

			var msgs []string
			for _, pack := range packs {
				vtui.ReplaceStrings(baseStrings)
				if len(pack.Strings) > 0 {
					vtui.AddStrings(pack.Strings)
				}

				errs := func() []error {
					// Re-init FrameManager and Screen for each language pass.
					scr := vtui.NewSilentScreenBuf()
					scr.AllocBuf(120, 60)
					manager := vtui.FrameManager
					manager.Init(scr)

					// Create and push a dummy PanelsFrame so context-aware actions find it.
					localVFS := vfs.NewOSVFS(tmpDir)
					_ = localVFS.SetPath(tmpDir)
					pf := NewPanelsFrame()
					left := NewFileSystemPanel(0, 0, 40, 20, localVFS)
					right := NewFileSystemPanel(40, 0, 40, 20, localVFS.Clone())
					waitForLoad(t, left)
					waitForLoad(t, right)
					pf.panels[0] = left
					pf.panels[1] = right
					pf.ResizeConsole(120, 60)
					waitForLoad(t, pf.panels[0].(*FileSystemPanel))
					waitForLoad(t, pf.panels[1].(*FileSystemPanel))
					manager.Push(pf)
					defer func() {
						waitForDirectoryLoads(t)
						// This includes the PanelsFrame and every dialog added by the action.
						closeFrameManagerFrames(manager)
					}()

					// Setup editor/viewer context if testing editor/viewer actions.
					if strings.HasPrefix(name, "Editor.") {
						showEditor(pf, localVFS, srcFile, &vfs.MemoryReadAtCloser{Data: []byte("dummy")})
					} else if strings.HasPrefix(name, "Viewer.") {
						vv, err := NewViewerView(context.Background(), localVFS, srcFile)
						if err == nil {
							showViewer(pf, vv, srcFile)
						}
					}

					initialCount := len(manager.Screens[manager.ActiveIdx].Frames)

					// Trigger the action handler.
					act.Handler()
					waitForLoad(t, pf.panels[0].(*FileSystemPanel))
					waitForLoad(t, pf.panels[1].(*FileSystemPanel))
					if manager.GetActiveToast() != "" {
						waitForToastExpiry(t, 6*time.Second)
					}

					frames := manager.Screens[manager.ActiveIdx].Frames
					if len(frames) <= initialCount {
						// Many actions are silent and do not open a dialog (e.g. Editor.Save).
						// This is completely expected, so skip validation for this pass.
						return nil
					}

					// Check if the top-most frame is a container. If not (e.g. raw drawing view), skip it safely.
					topFrame := frames[len(frames)-1]
					container, ok := topFrame.(vtui.Container)
					if !ok {
						return nil
					}
					return vtui.ValidateLayoutWithRules(container, rules)
				}()

				packName := pack.Name
				if packName == "" {
					packName = "?"
				}
				for _, err := range errs {
					msgs = append(msgs, fmt.Sprintf("[lang:%s] %s", packName, err))
				}
			}

			if len(msgs) > 0 {
				t.Errorf("Layout validation failed for Action %s:\n%s", name, strings.Join(msgs, "\n"))
			}
		})
	}
}
