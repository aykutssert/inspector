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

