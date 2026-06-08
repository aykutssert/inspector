// @ts-nocheck
import React, { useState } from "react";

export function InputsComponent() {
  const [val, setVal] = useState("");

  return (
    <div>
      {/* Violation: Both value and defaultValue */}
      <input value={val} defaultValue="default" />

      {/* Violation: Both value and defaultValue (reversed) */}
      <input defaultValue="default" value={val} />

      {/* Violation: Both checked and defaultChecked */}
      <input type="checkbox" checked={true} defaultChecked={false} />

      {/* Violation: Both checked and defaultChecked (reversed) */}
      <input type="checkbox" defaultChecked={false} checked={true} />

      {/* Violation: value without onChange/readOnly/disabled */}
      <input value={val} />

      {/* Violation: checked without onChange/readOnly/disabled */}
      <input type="checkbox" checked={true} />

      {/* Violation: textarea value without onChange/readOnly/disabled */}
      <textarea value={val} />

      {/* Violation: select value without onChange/readOnly/disabled */}
      <select value={val}>
        <option>A</option>
      </select>

      {/* Safe: value with onChange */}
      <input value={val} onChange={(e) => setVal(e.target.value)} />

      {/* Safe: value with readOnly */}
      <input value={val} readOnly />

      {/* Safe: value with readOnly={true} */}
      <input value={val} readOnly={true} />

      {/* Safe: value with disabled */}
      <input value={val} disabled />

      {/* Safe: value with disabled={true} */}
      <input value={val} disabled={true} />

      {/* Safe: input type button/submit/hidden/reset/image with value */}
      <input type="button" value="Click me" />
      <input type="submit" value="Submit form" />
      <input type="hidden" value="secret" />
      <input type="reset" value="Reset" />
      <input type="image" src="img.jpg" value="Image button" />

      {/* Safe: uncontrolled input using defaultValue */}
      <input defaultValue="Initial" />
    </div>
  );
}
