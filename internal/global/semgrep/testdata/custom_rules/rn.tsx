// @ts-nocheck — semgrep React Native golden fixture
import React from "react";
import { Dimensions, FlatList, Pressable, ScrollView, Text, TextInput, View, useWindowDimensions } from "react-native";
import { Dimensions as RNDimensions } from "react-native";
import * as RN from "react-native";
import AsyncStorage from "@react-native-async-storage/async-storage";

export function NativeDomElements() {
  return (
    <View>
      <div>Broken native layout</div>
      <span>Broken native text</span>
      <Text>Native text is fine</Text>
      <Pressable>
        <Text>Native button is fine</Text>
      </Pressable>
      <TextInput value="safe" />
    </View>
  );
}

const renderUser = ({ item }: any) => <Text>{item.name}</Text>;

export function NativeInlineRenderItem({ users }: any) {
  return (
    <View>
      <FlatList data={users} renderItem={({ item }) => <Text>{item.name}</Text>} />
      <FlatList data={users} renderItem={function ({ item }) { return <Text>{item.email}</Text>; }} />
      <FlatList data={users} renderItem={renderUser} />
    </View>
  );
}

export function NativeScrollViewList({ users }: any) {
  return (
    <ScrollView>
      {users.map((user: any) => (
        <Text key={user.id}>{user.name}</Text>
      ))}
    </ScrollView>
  );
}

export function NativeStaticScrollView() {
  return (
    <ScrollView>
      <Text>Static intro</Text>
      <Text>Static details</Text>
    </ScrollView>
  );
}

export async function NativeAsyncStorageSecrets(token: string) {
  await AsyncStorage.setItem("authToken", token);
  await AsyncStorage.setItem("session_id", token);
  await AsyncStorage.setItem("theme", "dark");
}

export function NativeFalsyAndRender({ items, count, isOpen }: any) {
  return (
    <View>
      {items.length && <Text>has items</Text>}{/* triggers rn-no-falsy-and-render */}
      {count && <Text>{count}</Text>}{/* triggers rn-no-falsy-and-render */}
      {items.length > 0 && <Text>ok</Text>}{/* ok — boolean comparison */}
      {isOpen && <Text>ok</Text>}{/* ok — non-numeric name */}
    </View>
  );
}

const initialWindow = Dimensions.get("window");

export function NativeResponsiveLayout() {
  const staleWindow = Dimensions.get("window");
  const aliasedWindow = RNDimensions.get("window");
  const namespacedWindow = RN.Dimensions.get("window");
  const currentWindow = useWindowDimensions();
  return <View style={{ width: currentWindow.width }}>{staleWindow.width + aliasedWindow.width + namespacedWindow.width + initialWindow.width}</View>;
}

export function ExpoEnvTest(key: string) {
  const a = process.env[key]; // should trigger expo-no-non-inlined-env
  const b = process.env.EXPO_PUBLIC_API_URL; // ok
  const c = process.env["EXPO_PUBLIC_API_URL"]; // ok
}

import { FlashList } from "@shopify/flash-list";

export function NativeFlashList({ data, renderRow }: any) {
  return (
    <View>
      <FlashList data={data} renderItem={renderRow} />{/* triggers rn-list-missing-estimated-item-size */}
      <FlashList data={data} renderItem={renderRow} estimatedItemSize={64} />{/* ok */}
    </View>
  );
}

// Violations: legacy Expo packages (triggers rn.rn-no-legacy-expo-packages)
import ExpoPermissions from "expo-permissions";
import * as AppLoading from "expo-app-loading";
const AdsAdmob = require("expo-ads-admob");

// Safe: modern Expo packages
import * as SplashScreen from "expo-splash-screen";
import { Image } from "expo-image";

// Violation: @gorhom/bottom-sheet → native Modal formSheet (triggers rn.rn-bottom-sheet-prefer-native)
import BottomSheet from "@gorhom/bottom-sheet";

// Violation: JS stack navigator → native-stack (triggers rn.rn-no-non-native-navigator)
import { createStackNavigator } from "@react-navigation/stack";

// Violation: Image from react-native → prefer expo-image (triggers rn.rn-prefer-expo-image)
import { Image as RNImage } from "react-native";

// Violation: TouchableOpacity → Pressable (triggers rn.rn-prefer-pressable)
import { TouchableOpacity, TouchableHighlight } from "react-native";

export function LegacyTouchables() {
  return (
    <View>
      <TouchableOpacity onPress={() => {}}><Text>Old button</Text></TouchableOpacity>
      <TouchableHighlight onPress={() => {}}><Text>Old highlight</Text></TouchableHighlight>
    </View>
  );
}

// Violation: deep internal import (triggers rn.rn-no-deep-imports)
import { something } from "react-native/Libraries/Utilities/Platform";

// Violation: PanResponder (triggers rn.rn-no-panresponder)
import { PanResponder, Animated, ScrollView, StyleSheet } from "react-native";

export function LegacyGestures() {
  const pan = PanResponder.create({
    onMoveShouldSetPanResponder: () => true,
    onPanResponderMove: () => {},
  });
  return <View {...pan.panHandlers} />;
}

// Violations: Animated API → prefer Reanimated (triggers rn.rn-prefer-reanimated)
export function LegacyAnimations() {
  const opacity = new Animated.Value(0);
  Animated.timing(opacity, { toValue: 1, duration: 300, useNativeDriver: true }).start();
  Animated.spring(opacity, { toValue: 1, useNativeDriver: true }).start();
  return <Animated.View style={{ opacity }} />;
}

// Violation: shadowColor in inline style (triggers rn.rn-no-legacy-shadow-styles)
export function ShadowBox() {
  return <View style={{ shadowColor: "#000", shadowOffset: { width: 0, height: 2 }, shadowRadius: 4, shadowOpacity: 0.3 }} />;
}

// Violation: StyleSheet with shadowColor (triggers rn.rn-no-legacy-shadow-styles)
const shadowStyles = StyleSheet.create({
  card: { shadowColor: "#000" },
});

// Violation: single-element style array (triggers rn.rn-no-single-element-style-array)
export function SingleArrayStyle() {
  return <View style={[shadowStyles.card]}><Text>Unnecessary array</Text></View>;
}

// Violation: contentContainerStyle flex:1 (triggers rn.rn-scrollview-flex-in-content-container)
export function FlexScrollView({ children }: any) {
  return (
    <ScrollView contentContainerStyle={{ flex: 1 }}>
      {children}
    </ScrollView>
  );
}

// Safe: flexGrow:1 is correct
export function FlexGrowScrollView({ children }: any) {
  return (
    <ScrollView contentContainerStyle={{ flexGrow: 1 }}>
      {children}
    </ScrollView>
  );
}

// Violation: key on renderItem root when keyExtractor present (triggers rn.rn-no-renderitem-key)
export function RedundantKeyList({ users }: any) {
  return (
    <FlatList
      data={users}
      keyExtractor={(item) => item.id}
      renderItem={({ item }) => <Text key={item.id}>{item.name}</Text>}
    />
  );
}

// Safe: no key on renderItem root (keyExtractor handles it)
export function SafeKeyList({ users }: any) {
  return (
    <FlatList
      data={users}
      keyExtractor={(item) => item.id}
      renderItem={({ item }) => <Text>{item.name}</Text>}
    />
  );
}

export function SetNativePropsTest(ref: any, inputRef: any) {
  // Violations: setNativeProps usage (triggers rn.rn-no-set-native-props)
  ref.setNativeProps({ text: "hello" });
  inputRef.current.setNativeProps({ style: { color: "red" } });

  // Safe: regular ref methods or state updates
  ref.current.focus();
  inputRef.current.clear();
}



