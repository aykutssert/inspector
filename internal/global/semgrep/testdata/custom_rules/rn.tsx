// @ts-nocheck — semgrep React Native golden fixture
import React from "react";
import { FlatList, Pressable, ScrollView, Text, TextInput, View } from "react-native";
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
