//go:build windows

package program

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPackageFamilyName(t *testing.T) {
	for _, test := range []struct {
		fullName string
		want     string
	}{
		{"Microsoft.WindowsCalculator_11.2210.0.0_x64__8wekyb3d8bbwe", "Microsoft.WindowsCalculator_8wekyb3d8bbwe"},
		{"Clipchamp.Clipchamp_4.5.10520.0_x64__yxz26nhyzhsrt", "Clipchamp.Clipchamp_yxz26nhyzhsrt"},
		{"1527c705-839a-4832-9118-54d4Bd6a0c89_10.0.19640.1000_neutral_neutral_cw5n1h2txyewy", "1527c705-839a-4832-9118-54d4Bd6a0c89_cw5n1h2txyewy"},
		{"something else", ""},
	} {
		if got := packageFamilyName(test.fullName); got != test.want {
			t.Errorf("packageFamilyName(%q) = %q, want %q", test.fullName, got, test.want)
		}
	}
}

// writePackage writes a package holding one app, and answers where it is.
func writePackage(t *testing.T, manifest string) string {
	t.Helper()

	location := t.TempDir()
	if err := os.WriteFile(filepath.Join(location, "AppxManifest.xml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	return location
}

const calculatorManifest = `<?xml version="1.0" encoding="utf-8"?>
<Package xmlns="http://schemas.microsoft.com/appx/manifest/foundation/windows10"
         xmlns:uap="http://schemas.microsoft.com/appx/manifest/uap/windows10"
         xmlns:uap10="http://schemas.microsoft.com/appx/manifest/uap/windows10/10">
  <Applications>
    <Application Id="App" Executable="Calculator.exe" EntryPoint="Windows.FullTrustApplication">
      <uap:VisualElements DisplayName="ms-resource:AppName" Description="ms-resource:AppDescription"
                          Square150x150Logo="Assets\Square150x150Logo.png"
                          Square44x44Logo="Assets\Square44x44Logo.png" BackgroundColor="transparent">
        <uap:DefaultTile Wide310x150Logo="Assets\Wide310x150Logo.png" />
      </uap:VisualElements>
    </Application>
  </Applications>
</Package>`

func TestAStoreAppIsReadFromItsPackage(t *testing.T) {
	location := writePackage(t, calculatorManifest)

	logo, description, canElevate := uwpManifest(location, "App")

	if logo != "" {
		t.Errorf("the logo is %q, want nothing: the package ships no pictures", logo)
	}
	// A name the package did not resolve is not worth showing.
	if description != "" {
		t.Errorf("the description is %q, want nothing for an unresolved resource", description)
	}
	if !canElevate {
		t.Errorf("the app is a full trust application, so it can run elevated")
	}
}

func TestAStoreAppIsReadWithItsLogo(t *testing.T) {
	location := writePackage(t, calculatorManifest)
	assets := filepath.Join(location, "Assets")
	if err := os.MkdirAll(assets, 0o755); err != nil {
		t.Fatal(err)
	}
	// Windows ships a picture for each screen scale rather than the one
	// the manifest names.
	for _, name := range []string{"Square44x44Logo.scale-400.png", "Square44x44Logo.scale-100.png"} {
		if err := os.WriteFile(filepath.Join(assets, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	logo, _, _ := uwpManifest(location, "App")
	if filepath.Base(logo) != "Square44x44Logo.scale-100.png" {
		t.Errorf("the logo is %q, want the one drawn for a screen scale of 100", logo)
	}
}

func TestAStoreAppSaysWhatItDoesWhenItCan(t *testing.T) {
	location := writePackage(t, `<?xml version="1.0" encoding="utf-8"?>
<Package xmlns="http://schemas.microsoft.com/appx/manifest/foundation/windows10"
         xmlns:uap="http://schemas.microsoft.com/appx/manifest/uap/windows10">
  <Applications>
    <Application Id="App" Executable="Thing.exe" EntryPoint="Thing.App">
      <uap:VisualElements DisplayName="Thing" Description="Does a thing" />
    </Application>
  </Applications>
</Package>`)

	_, description, canElevate := uwpManifest(location, "App")

	if description != "Does a thing" {
		t.Errorf("the description is %q, want what the package says", description)
	}
	if canElevate {
		t.Errorf("an app that is not a full trust application cannot run elevated")
	}
}

func TestTheRightAppOfAPackageIsRead(t *testing.T) {
	location := writePackage(t, `<?xml version="1.0" encoding="utf-8"?>
<Package xmlns="http://schemas.microsoft.com/appx/manifest/foundation/windows10"
         xmlns:uap="http://schemas.microsoft.com/appx/manifest/uap/windows10">
  <Applications>
    <Application Id="First" EntryPoint="First.App">
      <uap:VisualElements Description="The first" />
    </Application>
    <Application Id="Second" EntryPoint="Second.App">
      <uap:VisualElements Description="The second" />
    </Application>
  </Applications>
</Package>`)

	_, description, _ := uwpManifest(location, "Second")

	if description != "The second" {
		t.Errorf("the description is %q, want the second app's", description)
	}
}

func TestAStoreAppIsRunByItsModelID(t *testing.T) {
	program := Program{AppUserModelID: "Microsoft.WindowsCalculator_8wekyb3d8bbwe!App"}

	want := `shell:AppsFolder\Microsoft.WindowsCalculator_8wekyb3d8bbwe!App`
	if got := program.RunTarget(); got != want {
		t.Errorf("RunTarget() = %q, want %q", got, want)
	}

	// A program that is a file is still run as one.
	file := Program{FullPath: `C:\Windows\notepad.exe`}
	if got := file.RunTarget(); got != `C:\Windows\notepad.exe` {
		t.Errorf("RunTarget() = %q, want the file", got)
	}
}
