// @ts-nocheck
import { motion } from "framer-motion";

// ─── no-large-animated-blur ──────────────────────────────────────────────────

// Violation: large blur in Framer Motion animate prop
function HeavyBlurCard() {
  return (
    <motion.div
      initial={{ filter: "blur(0px)" }}
      animate={{ filter: "blur(40px)" }}
    >
      Card
    </motion.div>
  );
}

// Violation: large blur on hover
function HoverBlur() {
  return (
    <motion.div
      whileHover={{ filter: "blur(50px)" }}
    >
      Hover me
    </motion.div>
  );
}

// Safe: small blur (under threshold)
function SmallBlur() {
  return (
    <motion.div animate={{ filter: "blur(8px)" }}>Small blur</motion.div>
  );
}

// Safe: opacity animation (no blur)
function FadeIn() {
  return (
    <motion.div animate={{ opacity: 1 }}>Fade</motion.div>
  );
}

// ─── no-permanent-will-change ─────────────────────────────────────────────────

// Violation: will-change always active in static style
function AlwaysGPU() {
  return <div style={{ willChange: "transform" }}>GPU pinned forever</div>;
}

// Violation: will-change: all (worst case)
function AllGPU() {
  return <div style={{ willChange: "all", color: "red" }}>Bad</div>;
}

// Safe: conditional will-change (toggled by animation state)
function ConditionalGPU({ isAnimating }: { isAnimating: boolean }) {
  return (
    <div style={isAnimating ? { willChange: "transform" } : {}}>
      Conditional
    </div>
  );
}
