// trigger no-dynamic-import-path
const path = './features/heavy.js';
import(path);

// trigger no-dynamic-import-path
import(`./features/${name}`);

// safe: static string literal
import('./features/light.js');
import(".../features/light2.js");
import(`./features/light3.js`); // template literal without interpolation
