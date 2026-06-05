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
