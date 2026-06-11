import { useAnimatedReaction, useSharedValue, useDerivedValue } from "react-native-reanimated";

export function PureCopy() {
  const sv = useSharedValue(0);
  const progress = useSharedValue(0);

  // --- positive: reaction is a pure copy ---
  useAnimatedReaction(
    () => progress.value,
    (current) => {
      sv.value = current; // FIRE
    },
  );

  return sv;
}

export function RealReaction() {
  const sv = useSharedValue(0);
  const progress = useSharedValue(0);

  // --- negative: reaction has a side-effect / transform ---
  useAnimatedReaction(
    () => progress.value,
    (current, previous) => {
      if (current !== previous) {
        sv.value = current * 2; // NO FIRE (transform + guard)
      }
    },
  );

  // --- negative: already useDerivedValue ---
  const derived = useDerivedValue(() => progress.value); // NO FIRE
  return derived;
}
