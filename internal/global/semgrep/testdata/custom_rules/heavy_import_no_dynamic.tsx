// Violation 1: static import of monaco-editor
import * as monaco from 'monaco-editor';

// Violation 2: static import of three
import { Scene } from 'three';

// Violation 3: static import of lodash
import _ from 'lodash';

// Safe 1: static import of standard library (e.g. react)
import React from 'react';

// Safe 2: dynamic import of heavy library
const DynamicEditor = () => {
  import('monaco-editor').then((m) => {
    console.log(m);
  });
};

// Violation 4: static import of recharts (triggers general.prefer-dynamic-import)
import Recharts from 'recharts';

// Violation 5: static import of apexcharts (triggers general.prefer-dynamic-import)
import ApexCharts from 'apexcharts';

// Violation 6: static import of pdfjs-dist (triggers general.prefer-dynamic-import)
import 'pdfjs-dist';

// Safe 3: dynamic import of recharts
const DynamicChart = () => {
  import('recharts').then((c) => {
    console.log(c);
  });
};

// Violations: eager motion imports (triggers general.use-lazy-motion)
import { motion } from "framer-motion";
import { AnimatePresence, motion as motionElement } from "framer-motion";
import eagerMotion from "framer-motion";

// Safe: lazy motion imports
import { m, LazyMotion, domAnimation } from "framer-motion";


