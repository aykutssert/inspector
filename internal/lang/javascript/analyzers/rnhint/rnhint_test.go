package rnhint

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/aykutssert/scout/internal/core"
)

func scanSource(t *testing.T, name, src string) []core.Finding {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	findings, err := scanFile(path, name)
	if err != nil {
		t.Fatal(err)
	}
	return findings
}

func TestImageChildrenDetected(t *testing.T) {
	src := `
		import { Image, Text } from "react-native";
		export function Avatar() {
			return <Image source={{ uri: "avatar" }}><Text>Name</Text></Image>;
		}
	`
	findings := scanSource(t, "Avatar.tsx", src)
	if len(findings) != 1 || findings[0].RuleID != "rn-no-image-children" || findings[0].Severity != core.SeverityError {
		t.Fatalf("expected deterministic Image children finding, got %#v", findings)
	}
}

func TestAliasedAndNamespaceImageDetected(t *testing.T) {
	src := `
		import { Image as RNImage, Text } from "react-native";
		import * as RN from "react-native";
		export const A = () => <RNImage><Text>A</Text></RNImage>;
		export const B = () => <RN.Image>{content}</RN.Image>;
	`
	findings := scanSource(t, "Images.tsx", src)
	if len(findings) != 2 {
		t.Fatalf("expected alias and namespace findings, got %#v", findings)
	}
}

func TestSafeImageFormsStayQuiet(t *testing.T) {
	src := `
		import { Image, ImageBackground, Text } from "react-native";
		const LocalImage = ({ children }) => <>{children}</>;
		export const A = () => <Image source={{ uri: "avatar" }} />;
		export const B = () => <Image></Image>;
		export const C = () => <Image>{/* no child */}</Image>;
		export const D = () => <ImageBackground><Text>Overlay</Text></ImageBackground>;
		export const E = () => <LocalImage><Text>Local</Text></LocalImage>;
	`
	if findings := scanSource(t, "Safe.tsx", src); len(findings) != 0 {
		t.Fatalf("safe Image forms must stay quiet, got %#v", findings)
	}
}

func TestUnboundWebImageStaysQuiet(t *testing.T) {
	src := `
		const Image = ({ children }) => <figure>{children}</figure>;
		export const Card = () => <Image><span>Web</span></Image>;
	`
	if findings := scanSource(t, "Card.tsx", src); len(findings) != 0 {
		t.Fatalf("unbound Image must stay quiet, got %#v", findings)
	}
}

func TestBareStringOutsideText(t *testing.T) {
	src := `
		import { View, TouchableOpacity as Touch } from "react-native";
		export function App() {
			return (
				<View>
					Hello Raw Text
					<Touch>
						{"Hello Expression"}
					</Touch>
				</View>
			);
		}
	`
	findings := scanSource(t, "App.tsx", src)
	if len(findings) != 2 {
		t.Fatalf("expected exactly 2 bare string findings, got %d: %#v", len(findings), findings)
	}

	f1, f2 := findings[0], findings[1]
	if f1.RuleID != "rn-bare-string-outside-text" || f2.RuleID != "rn-bare-string-outside-text" {
		t.Fatalf("expected rn-bare-string-outside-text findings, got rule IDs: %s, %s", f1.RuleID, f2.RuleID)
	}
	if f1.Line != 5 || f2.Line != 7 {
		t.Fatalf("expected findings on lines 5 and 7, got lines: %d, %d", f1.Line, f2.Line)
	}
}

func TestSafeTextInsideText(t *testing.T) {
	src := `
		import { View, Text } from "react-native";
		import * as RN from "react-native";
		export function App() {
			return (
				<View>
					<Text>Safe Text</Text>
					<RN.Text>Safe Namespaced Text</RN.Text>
				</View>
			);
		}
	`
	findings := scanSource(t, "App.tsx", src)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got: %#v", findings)
	}
}

func TestNonReactNativeFilesStayQuiet(t *testing.T) {
	src := `
		import { View } from "custom-web-lib";
		export function WebApp() {
			return <View>Web Bare Text</View>;
		}
	`
	findings := scanSource(t, "WebApp.tsx", src)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for non-React Native file, got: %#v", findings)
	}
}

func TestListDataMapped(t *testing.T) {
	src := `
		import { FlatList } from "react-native";
		export function App({ items }) {
			return (
				<FlatList
					data={items.map(x => x.id)}
					renderItem={renderItem}
				/>
			);
		}
	`
	findings := scanSource(t, "App.tsx", src)
	if len(findings) != 1 || findings[0].RuleID != "rn-list-data-mapped" {
		t.Fatalf("expected rn-list-data-mapped finding, got %#v", findings)
	}
}

func TestListCallbackPerRow(t *testing.T) {
	src := `
		import { FlatList } from "react-native";
		export function App({ items }) {
			return (
				<FlatList
					data={items}
					renderItem={({ item }) => <Item item={item} />}
				/>
			);
		}
	`
	findings := scanSource(t, "App.tsx", src)
	if len(findings) != 1 || findings[0].RuleID != "rn-list-callback-per-row" {
		t.Fatalf("expected rn-list-callback-per-row finding, got %#v", findings)
	}
}

func TestScrollViewMappedList(t *testing.T) {
	src := `
		import { ScrollView } from "react-native";
		export function App({ items }) {
			return (
				<ScrollView>
					{items.map(item => <Item item={item} />)}
				</ScrollView>
			);
		}
	`
	findings := scanSource(t, "App.tsx", src)
	if len(findings) != 1 || findings[0].RuleID != "rn-no-scrollview-mapped-list" {
		t.Fatalf("expected rn-no-scrollview-mapped-list finding, got %#v", findings)
	}
}

func TestRNPerformanceSafe(t *testing.T) {
	src := `
		import { FlatList, ScrollView } from "react-native";
		const renderItem = ({ item }) => <Item item={item} />;
		export function App({ items, preparedData }) {
			return (
				<>
					<FlatList
						data={preparedData}
						renderItem={renderItem}
					/>
					<ScrollView>
						{items}
					</ScrollView>
				</>
			);
		}
	`
	findings := scanSource(t, "App.tsx", src)
	if len(findings) != 0 {
		t.Fatalf("did not expect any performance findings, got: %#v", findings)
	}
}

