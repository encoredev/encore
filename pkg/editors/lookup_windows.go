//go:build windows

package editors

import (
	"context"
	"fmt"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sys/windows/registry"

	"encr.dev/pkg/fns"
)

type WindowsExternalEditor struct {
	Name                       EditorName
	RegistryKeys               []RegistryKey
	DisplayNamePrefixes        []string
	Publishers                 []string
	JetBrainsToolboxScriptName string
	InstallLocationRegistryKey string
	ExecutableShimPaths        []string
}

type RegistryKey struct {
	Key    registry.Key
	SubKey string
}

type RegistryValue struct {
	Name        string
	Type        uint32
	StringValue string
}

var editors = []WindowsExternalEditor{
	{
		Name:                Atom,
		RegistryKeys:        []RegistryKey{CurrentUserUninstallKey("atom")},
		ExecutableShimPaths: []string{"bin/atom.cmd"},
		DisplayNamePrefixes: []string{"Atom"},
		Publishers:          []string{"GitHub Inc."},
	},
	{
		Name:                AtomBeta,
		RegistryKeys:        []RegistryKey{CurrentUserUninstallKey("atom-beta")},
		ExecutableShimPaths: []string{"bin/atom-beta.cmd"},
		DisplayNamePrefixes: []string{"Atom Beta"},
		Publishers:          []string{"GitHub Inc."},
	},
	{
		Name:                AtomNightly,
		RegistryKeys:        []RegistryKey{CurrentUserUninstallKey("atom-nightly")},
		ExecutableShimPaths: []string{"bin/atom-nightly.cmd"},
		DisplayNamePrefixes: []string{"Atom Nightly"},
		Publishers:          []string{"GitHub Inc."},
	},
	{
		Name: VSCode,
		RegistryKeys: []RegistryKey{
			// 64-bit version of VSCode (user) - provided by default in 64-bit Windows
			CurrentUserUninstallKey("{771FD6B0-FA20-440A-A002-3B3BAC16DC50}_is1"),
			// 32-bit version of VSCode (user)
			CurrentUserUninstallKey("{D628A17A-9713-46BF-8D57-E671B46A741E}_is1"),
			// ARM64 version of VSCode (user)
			CurrentUserUninstallKey("{D9E514E7-1A56-452D-9337-2990C0DC4310}_is1"),
			// 64-bit version of VSCode (system) - was default before user scope installation
			LocalMachineUninstallKey("{EA457B21-F73E-494C-ACAB-524FDE069978}_is1"),
			// 32-bit version of VSCode (system)
			Wow64LocalMachineUninstallKey("{F8A2A208-72B3-4D61-95FC-8A65D340689B}_is1"),
			// ARM64 version of VSCode (system)
			LocalMachineUninstallKey("{A5270FC5-65AD-483E-AC30-2C276B63D0AC}_is1"),
		},
		ExecutableShimPaths: []string{"bin/code.cmd"},
		DisplayNamePrefixes: []string{"Microsoft Visual Studio Code"},
		Publishers:          []string{"Microsoft Corporation"},
	},
	{
		Name: VSCodeInsiders,
		RegistryKeys: []RegistryKey{
			// 64-bit version of VSCode (user) - provided by default in 64-bit Windows
			CurrentUserUninstallKey("{217B4C08-948D-4276-BFBB-BEE930AE5A2C}_is1"),
			// 32-bit version of VSCode (user)
			CurrentUserUninstallKey("{26F4A15E-E392-4887-8C09-7BC55712FD5B}_is1"),
			// ARM64 version of VSCode (user)
			CurrentUserUninstallKey("{69BD8F7B-65EB-4C6F-A14E-44CFA83712C0}_is1"),
			// 64-bit version of VSCode (system) - was default before user scope installation
			LocalMachineUninstallKey("{1287CAD5-7C8D-410D-88B9-0D1EE4A83FF2}_is1"),
			// 32-bit version of VSCode (system)
			Wow64LocalMachineUninstallKey("{C26E74D1-022E-4238-8B9D-1E7564A36CC9}_is1"),
			// ARM64 version of VSCode (system)
			LocalMachineUninstallKey("{0AEDB616-9614-463B-97D7-119DD86CCA64}_is1"),
		},
		ExecutableShimPaths: []string{"bin/code-insiders.cmd"},
		DisplayNamePrefixes: []string{"Microsoft Visual Studio Code Insiders"},
		Publishers:          []string{"Microsoft Corporation"},
	},
	{
		Name: VSCodium,
		RegistryKeys: []RegistryKey{
			// 64-bit version of VSCodium (user)
			CurrentUserUninstallKey("{2E1F05D1-C245-4562-81EE-28188DB6FD17}_is1"),
			// 32-bit version of VSCodium (user) - new key
			CurrentUserUninstallKey("{0FD05EB4-651E-4E78-A062-515204B47A3A}_is1"),
			// ARM64 version of VSCodium (user) - new key
			CurrentUserUninstallKey("{57FD70A5-1B8D-4875-9F40-C5553F094828}_is1"),
			// 64-bit version of VSCodium (system) - new key
			LocalMachineUninstallKey("{88DA3577-054F-4CA1-8122-7D820494CFFB}_is1"),
			// 32-bit version of VSCodium (system) - new key
			Wow64LocalMachineUninstallKey("{763CBF88-25C6-4B10-952F-326AE657F16B}_is1"),
			// ARM64 version of VSCodium (system) - new key
			LocalMachineUninstallKey("{67DEE444-3D04-4258-B92A-BC1F0FF2CAE4}_is1"),
			// 32-bit version of VSCodium (user) - old key
			CurrentUserUninstallKey("{C6065F05-9603-4FC4-8101-B9781A25D88E}}_is1"),
			// ARM64 version of VSCodium (user) - old key
			CurrentUserUninstallKey("{3AEBF0C8-F733-4AD4-BADE-FDB816D53D7B}_is1"),
			// 64-bit version of VSCodium (system) - old key
			LocalMachineUninstallKey("{D77B7E06-80BA-4137-BCF4-654B95CCEBC5}_is1"),
			// 32-bit version of VSCodium (system) - old key
			Wow64LocalMachineUninstallKey("{E34003BB-9E10-4501-8C11-BE3FAA83F23F}_is1"),
			// ARM64 version of VSCodium (system) - old key
			LocalMachineUninstallKey("{D1ACE434-89C5-48D1-88D3-E2991DF85475}_is1"),
		},
		ExecutableShimPaths: []string{"bin/codium.cmd"},
		DisplayNamePrefixes: []string{"VSCodium"},
		Publishers:          []string{"VSCodium", "Microsoft Corporation"},
	},
	{
		Name: VSCodiumInsiders,
		RegistryKeys: []RegistryKey{
			// 64-bit version of VSCodium - Insiders (user)
			CurrentUserUninstallKey("{20F79D0D-A9AC-4220-9A81-CE675FFB6B41}_is1"),
			// 32-bit version of VSCodium - Insiders (user)
			CurrentUserUninstallKey("{ED2E5618-3E7E-4888-BF3C-A6CCC84F586F}_is1"),
			// ARM64 version of VSCodium - Insiders (user)
			CurrentUserUninstallKey("{2E362F92-14EA-455A-9ABD-3E656BBBFE71}_is1"),
			// 64-bit version of VSCodium - Insiders (system)
			LocalMachineUninstallKey("{B2E0DDB2-120E-4D34-9F7E-8C688FF839A2}_is1"),
			// 32-bit version of VSCodium - Insiders (system)
			Wow64LocalMachineUninstallKey("{EF35BB36-FA7E-4BB9-B7DA-D1E09F2DA9C9}_is1"),
			// ARM64 version of VSCodium - Insiders (system)
			LocalMachineUninstallKey("{44721278-64C6-4513-BC45-D48E07830599}_is1"),
		},
		ExecutableShimPaths: []string{"bin/codium-insiders.cmd"},
		DisplayNamePrefixes: []string{"VSCodium Insiders", "VSCodium (Insiders)"},
		Publishers:          []string{"VSCodium"},
	},
	{
		Name: SublimeText,
		RegistryKeys: []RegistryKey{
			// Sublime Text 4 (and newer?)
			LocalMachineUninstallKey("Sublime Text_is1"),
			// Sublime Text 3
			LocalMachineUninstallKey("Sublime Text 3_is1"),
		},
		ExecutableShimPaths: []string{"subl.exe"},
		DisplayNamePrefixes: []string{"Sublime Text"},
		Publishers:          []string{"Sublime HQ Pty Ltd"},
	},
	{
		Name: Brackets,
		RegistryKeys: []RegistryKey{
			Wow64LocalMachineUninstallKey("{4F3B6E8C-401B-4EDE-A423-6481C239D6FF}"),
		},
		ExecutableShimPaths: []string{"Brackets.exe"},
		DisplayNamePrefixes: []string{"Brackets"},
		Publishers:          []string{"brackets.io"},
	},
	{
		Name: ColdFusionBuilder,
		RegistryKeys: []RegistryKey{
			// 64-bit version of ColdFusionBuilder3
			LocalMachineUninstallKey("Adobe ColdFusion Builder 3_is1"),
			// 64-bit version of ColdFusionBuilder2016
			LocalMachineUninstallKey("Adobe ColdFusion Builder 2016"),
		},
		ExecutableShimPaths: []string{"CFBuilder.exe"},
		DisplayNamePrefixes: []string{"Adobe ColdFusion Builder"},
		Publishers:          []string{"Adobe Systems Incorporated"},
	},
	{
		Name: Typora,
		RegistryKeys: []RegistryKey{
			// 64-bit version of Typora
			LocalMachineUninstallKey("{37771A20-7167-44C0-B322-FD3E54C56156}_is1"),
			// 32-bit version of Typora
			Wow64LocalMachineUninstallKey("{37771A20-7167-44C0-B322-FD3E54C56156}_is1"),
		},
		ExecutableShimPaths: []string{"typora.exe"},
		DisplayNamePrefixes: []string{"Typora"},
		Publishers:          []string{"typora.io"},
	},
	{
		Name: SlickEdit,
		RegistryKeys: []RegistryKey{
			// 64-bit version of SlickEdit Pro 2018
			LocalMachineUninstallKey("{18406187-F49E-4822-CAF2-1D25C0C83BA2}"),
			// 32-bit version of SlickEdit Pro 2018
			Wow64LocalMachineUninstallKey("{18006187-F49E-4822-CAF2-1D25C0C83BA2}"),
			// 64-bit version of SlickEdit Standard 2018
			LocalMachineUninstallKey("{18606187-F49E-4822-CAF2-1D25C0C83BA2}"),
			// 32-bit version of SlickEdit Standard 2018
			Wow64LocalMachineUninstallKey("{18206187-F49E-4822-CAF2-1D25C0C83BA2}"),
			// 64-bit version of SlickEdit Pro 2017
			LocalMachineUninstallKey("{15406187-F49E-4822-CAF2-1D25C0C83BA2}"),
			// 32-bit version of SlickEdit Pro 2017
			Wow64LocalMachineUninstallKey("{15006187-F49E-4822-CAF2-1D25C0C83BA2}"),
			// 64-bit version of SlickEdit Pro 2016 (21.0.1)
			LocalMachineUninstallKey("{10C06187-F49E-4822-CAF2-1D25C0C83BA2}"),
			// 64-bit version of SlickEdit Pro 2016 (21.0.0)
			LocalMachineUninstallKey("{10406187-F49E-4822-CAF2-1D25C0C83BA2}"),
			// 64-bit version of SlickEdit Pro 2015 (20.0.3)
			LocalMachineUninstallKey("{0DC06187-F49E-4822-CAF2-1D25C0C83BA2}"),
			// 64-bit version of SlickEdit Pro 2015 (20.0.2)
			LocalMachineUninstallKey("{0D406187-F49E-4822-CAF2-1D25C0C83BA2}"),
			// 64-bit version of SlickEdit Pro 2014 (19.0.2)
			LocalMachineUninstallKey("{7CC0E567-ACD6-41E8-95DA-154CEEDB0A18}"),
		},
		ExecutableShimPaths: []string{"win/vs.exe"},
		DisplayNamePrefixes: []string{"SlickEdit"},
		Publishers:          []string{"SlickEdit Inc."},
	},
	{
		Name: AptanaStudio,
		RegistryKeys: []RegistryKey{
			Wow64LocalMachineUninstallKey("{2D6C1116-78C6-469C-9923-3E549218773F}"),
		},
		ExecutableShimPaths: []string{"AptanaStudio3.exe"},
		DisplayNamePrefixes: []string{"Aptana Studio"},
		Publishers:          []string{"Appcelerator"},
	},
	{
		Name:                       JetbrainsWebStorm,
		RegistryKeys:               registryKeysForJetBrainsIDE("WebStorm"),
		ExecutableShimPaths:        executableShimPathsForJetBrainsIDE("webstorm"),
		JetBrainsToolboxScriptName: "webstorm",
		DisplayNamePrefixes:        []string{"WebStorm"},
		Publishers:                 []string{"JetBrains s.r.o."},
	},
	{
		Name:                       JetbrainsPhpStorm,
		RegistryKeys:               registryKeysForJetBrainsIDE("PhpStorm"),
		ExecutableShimPaths:        executableShimPathsForJetBrainsIDE("phpstorm"),
		JetBrainsToolboxScriptName: "phpstorm",
		DisplayNamePrefixes:        []string{"PhpStorm"},
		Publishers:                 []string{"JetBrains s.r.o."},
	},
	{
		Name:                       AndroidStudio,
		RegistryKeys:               []RegistryKey{LocalMachineUninstallKey("Android Studio")},
		InstallLocationRegistryKey: "UninstallString",
		JetBrainsToolboxScriptName: "studio",
		ExecutableShimPaths: []string{
			"../bin/studio64.exe",
			"../bin/studio.exe",
		},
		DisplayNamePrefixes: []string{"Android Studio"},
		Publishers:          []string{"Google LLC"},
	},
	{
		Name: NotePadPlusPlus,
		RegistryKeys: []RegistryKey{
			// 64-bit version of Notepad++
			LocalMachineUninstallKey("Notepad++"),
			// 32-bit version of Notepad++
			Wow64LocalMachineUninstallKey("Notepad++"),
		},
		InstallLocationRegistryKey: "DisplayIcon",
		DisplayNamePrefixes:        []string{"Notepad++"},
		Publishers:                 []string{"Notepad++ Team"},
	},
	{
		Name:                       JetbrainsRider,
		RegistryKeys:               registryKeysForJetBrainsIDE("JetBrains Rider"),
		ExecutableShimPaths:        executableShimPathsForJetBrainsIDE("rider"),
		JetBrainsToolboxScriptName: "rider",
		DisplayNamePrefixes:        []string{"JetBrains Rider"},
		Publishers:                 []string{"JetBrains s.r.o."},
	},
	{
		Name:                       RStudio,
		RegistryKeys:               []RegistryKey{Wow64LocalMachineUninstallKey("RStudio")},
		InstallLocationRegistryKey: "DisplayIcon",
		DisplayNamePrefixes:        []string{"RStudio"},
		Publishers:                 []string{"RStudio", "Posit Software"},
	},
	{
		Name:                       JetbrainsIntelliJ,
		RegistryKeys:               registryKeysForJetBrainsIDE("IntelliJ IDEA"),
		ExecutableShimPaths:        executableShimPathsForJetBrainsIDE("idea"),
		JetBrainsToolboxScriptName: "idea",
		DisplayNamePrefixes:        []string{"IntelliJ IDEA "},
		Publishers:                 []string{"JetBrains s.r.o."},
	},
	{
		Name:                JetbrainsIntelliJCE,
		RegistryKeys:        registryKeysForJetBrainsIDE("IntelliJ IDEA Community Edition"),
		ExecutableShimPaths: executableShimPathsForJetBrainsIDE("idea"),
		DisplayNamePrefixes: []string{"IntelliJ IDEA Community Edition "},
		Publishers:          []string{"JetBrains s.r.o."},
	},
	{
		Name:                       JetbrainsPyCharm,
		RegistryKeys:               registryKeysForJetBrainsIDE("PyCharm"),
		ExecutableShimPaths:        executableShimPathsForJetBrainsIDE("pycharm"),
		JetBrainsToolboxScriptName: "pycharm",
		DisplayNamePrefixes:        []string{"PyCharm "},
		Publishers:                 []string{"JetBrains s.r.o."},
	},
	{
		Name:                JetbrainsPyCharmCE,
		RegistryKeys:        registryKeysForJetBrainsIDE("PyCharm Community Edition"),
		ExecutableShimPaths: executableShimPathsForJetBrainsIDE("pycharm"),
		DisplayNamePrefixes: []string{"PyCharm Community Edition"},
		Publishers:          []string{"JetBrains s.r.o."},
	},
	{
		Name:                       JetbrainsCLion,
		RegistryKeys:               registryKeysForJetBrainsIDE("CLion"),
		ExecutableShimPaths:        executableShimPathsForJetBrainsIDE("clion"),
		JetBrainsToolboxScriptName: "clion",
		DisplayNamePrefixes:        []string{"CLion "},
		Publishers:                 []string{"JetBrains s.r.o."},
	},
	{
		Name:                       JetbrainsRubyMine,
		RegistryKeys:               registryKeysForJetBrainsIDE("RubyMine"),
		ExecutableShimPaths:        executableShimPathsForJetBrainsIDE("rubymine"),
		JetBrainsToolboxScriptName: "rubymine",
		DisplayNamePrefixes:        []string{"RubyMine "},
		Publishers:                 []string{"JetBrains s.r.o."},
	},
	{
		Name:                       JetbrainsGoLand,
		RegistryKeys:               registryKeysForJetBrainsIDE("GoLand"),
		ExecutableShimPaths:        executableShimPathsForJetBrainsIDE("goland"),
		JetBrainsToolboxScriptName: "goland",
		DisplayNamePrefixes:        []string{"GoLand "},
		Publishers:                 []string{"JetBrains s.r.o."},
	},
	{
		Name:                       JetbrainsFleet,
		RegistryKeys:               []RegistryKey{LocalMachineUninstallKey("Fleet")},
		JetBrainsToolboxScriptName: "fleet",
		InstallLocationRegistryKey: "DisplayIcon",
		DisplayNamePrefixes:        []string{"Fleet "},
		Publishers:                 []string{"JetBrains s.r.o."},
	},
	{
		Name:                       JetbrainsDataSpell,
		RegistryKeys:               registryKeysForJetBrainsIDE("DataSpell"),
		ExecutableShimPaths:        executableShimPathsForJetBrainsIDE("dataspell"),
		JetBrainsToolboxScriptName: "dataspell",
		DisplayNamePrefixes:        []string{"DataSpell "},
		Publishers:                 []string{"JetBrains s.r.o."},
	},
	{
		Name: Pulsar,
		RegistryKeys: []RegistryKey{
			CurrentUserUninstallKey("0949b555-c22c-56b7-873a-a960bdefa81f"),
			LocalMachineUninstallKey("0949b555-c22c-56b7-873a-a960bdefa81f"),
		},
		ExecutableShimPaths: []string{"../pulsar/Pulsar.exe"},
		DisplayNamePrefixes: []string{"Pulsar"},
		Publishers:          []string{"Pulsar-Edit"},
	},
}

func registryKey(key registry.Key, subKey string) RegistryKey {
	return RegistryKey{Key: key, SubKey: subKey}
}

func CurrentUserUninstallKey(subKey string) RegistryKey {
	return registryKey(registry.CURRENT_USER, "Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\"+subKey)
}

func LocalMachineUninstallKey(subKey string) RegistryKey {
	return registryKey(registry.LOCAL_MACHINE, "Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\"+subKey)
}

func Wow64LocalMachineUninstallKey(subKey string) RegistryKey {
	return registryKey(registry.LOCAL_MACHINE, "Software\\Wow6432Node\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\"+subKey)
}

// This function generates registry keys for a given JetBrains product for the
// last 2 years, assuming JetBrains make no more than 5 major releases and
// no more than 5 minor releases per year.
func registryKeysForJetBrainsIDE(product string) []RegistryKey {
	const maxMajorReleasesPerYear = 5
	const maxMinorReleasesPerYear = 5
	var lastYear = time.Now().Year()
	var firstYear = lastYear - 2

	var result []RegistryKey

	for year := firstYear; year <= lastYear; year++ {
		for majorRelease := 1; majorRelease <= maxMajorReleasesPerYear; majorRelease++ {
			for minorRelease := 0; minorRelease <= maxMinorReleasesPerYear; minorRelease++ {
				key := fmt.Sprintf("%s %d.%d", product, year, majorRelease)
				if minorRelease > 0 {
					key = fmt.Sprintf("%s.%d", key, minorRelease)
				}

				result = append(result,
					Wow64LocalMachineUninstallKey(key),
					CurrentUserUninstallKey(key),
				)
			}
		}
	}

	// Return in reverse order to prioritize newer versions
	slices.Reverse(result)
	return result
}

// JetBrains IDE's might have 64 and/or 32 bit executables, so let's add both
func executableShimPathsForJetBrainsIDE(baseName string) []string {
	return []string{
		fmt.Sprintf("bin/%s64.exe", baseName),
		fmt.Sprintf("bin/%s.exe", baseName),
	}
}

func getKeyOrEmpty(keys []RegistryValue, valueName string) string {
	for _, key := range keys {
		if key.Name == valueName {
			if key.Type == registry.SZ {
				return key.StringValue
			}
		}
	}
	return ""
}

func getAppInfo(editor WindowsExternalEditor, keys []RegistryValue) (displayName, publisher, installLocation string) {
	displayName = getKeyOrEmpty(keys, "DisplayName")
	publisher = getKeyOrEmpty(keys, "Publisher")
	loc := editor.InstallLocationRegistryKey
	if loc == "" {
		loc = "InstallLocation"
	}
	installLocation = getKeyOrEmpty(keys, loc)
	return
}

func findApplication(ctx context.Context, editor WindowsExternalEditor, foundEditors chan FoundEditor) error {
	for _, registryKey := range editor.RegistryKeys {
		values, err := enumerateValues(registryKey.Key, registryKey.SubKey)
		if err != nil {
			return errors.Wrap(err, "failed to enumerate registry values")
		}
		if len(values) == 0 {
			continue
		}

		displayName, publisher, installLocation := getAppInfo(editor, values)

		if !validateStartsWith(displayName, editor.DisplayNamePrefixes) ||
			!stringInSlice(publisher, editor.Publishers) {
			log.Warn().Str("editor", string(editor.Name)).Str("publisher", publisher).Str("display_name", displayName).Msg("unexpected registry entry")
			continue
		}

		var executableShimPaths []string
		if editor.InstallLocationRegistryKey == "DisplayIcon" {
			executableShimPaths = []string{installLocation}
		} else {
			for _, shim := range editor.ExecutableShimPaths {
				executableShimPaths = append(executableShimPaths, filepath.Join(installLocation, shim))
			}
		}

		for _, exePath := range executableShimPaths {
			if pathExists(exePath) {
				foundEditors <- FoundEditor{
					Editor:    editor.Name,
					Path:      exePath,
					UsesShell: strings.HasSuffix(exePath, ".cmd"),
				}
				return nil
			} else {
				log.Debug().Str("editor", string(editor.Name)).Str("path", exePath).Msg("executable not found")
			}
		}
	}

	return findJetBrainsToolboxApplication(ctx, editor, foundEditors)
}

// Find JetBrain products installed through JetBrains Toolbox
func findJetBrainsToolboxApplication(_ context.Context, editor WindowsExternalEditor, foundEditors chan FoundEditor) error {
	if editor.JetBrainsToolboxScriptName == "" {
		return nil
	}

	var toolboxRegistryReference = []RegistryKey{
		CurrentUserUninstallKey("toolbox"),
		Wow64LocalMachineUninstallKey("toolbox"),
	}

	for _, registryKey := range toolboxRegistryReference {
		keys, err := enumerateValues(registryKey.Key, registryKey.SubKey)
		if err != nil {
			return errors.Wrap(err, "failed to enumerate registry values")
		}

		if len(keys) > 0 {
			editorPathInToolbox := path.Join(
				getKeyOrEmpty(keys, "UninstallString"),
				"..",
				"..",
				"scripts",
				fmt.Sprintf("%s.cmd", editor.JetBrainsToolboxScriptName),
			)
			if pathExists(editorPathInToolbox) {
				foundEditors <- FoundEditor{
					Editor:    editor.Name,
					Path:      editorPathInToolbox,
					UsesShell: true,
				}
			}
		}
	}

	return nil
}

func validateStartsWith(registryVal string, definedVal []string) bool {
	for _, val := range definedVal {
		if strings.HasPrefix(registryVal, val) {
			return true
		}
	}
	return false
}

func enumerateValues(key registry.Key, subKey string) ([]RegistryValue, error) {
	k, err := registry.OpenKey(key, subKey, registry.ENUMERATE_SUB_KEYS|registry.QUERY_VALUE)
	if err != nil {
		return nil, err
	}
	defer fns.CloseIgnore(k)

	valueNames, err := k.ReadValueNames(-1) // read all value names
	if err != nil {
		return nil, err
	}

	var values []RegistryValue
	for _, valueName := range valueNames {
		value, valueType, err := k.GetStringValue(valueName)
		if err != nil {
			if !errors.Is(err, registry.ErrUnexpectedType) && !errors.Is(err, registry.ErrNotExist) {
				return nil, err
			}
		} else {
			values = append(values, RegistryValue{Name: valueName, Type: valueType, StringValue: value})
		}
	}

	return values, nil
}

func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

// Resolve a list of installed editors on the user's machine, using known
// install and uninstall path registry keys
func getAvailableEditors(ctx context.Context) ([]FoundEditor, error) {
	results := make([]FoundEditor, 0)

	grp, ctx := errgroup.WithContext(ctx)

	foundEditors := make(chan FoundEditor)
	errs := make(chan error, 1)
	for _, editor := range editors {
		editor := editor
		grp.Go(func() error {
			return findApplication(ctx, editor, foundEditors)
		})
	}

	go func() {
		errs <- grp.Wait()
		close(foundEditors)
	}()

	// Collect results and the error from the group
	for editor := range foundEditors {
		results = append(results, editor)
	}
	if err := <-errs; err != nil {
		return nil, errors.WithStack(err)
	}

	return results, nil
}
