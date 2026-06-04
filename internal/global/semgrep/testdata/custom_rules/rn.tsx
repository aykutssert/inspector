// @ts-nocheck — semgrep React Native golden fixture
import React from "react";
import { FlatList, Pressable, ScrollView, Text, TextInput, View } from "react-native";

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
