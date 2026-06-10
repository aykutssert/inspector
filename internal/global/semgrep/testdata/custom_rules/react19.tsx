// @ts-nocheck
import React, { forwardRef, useEffect, useState } from "react";
import ReactDOM from "react-dom";

function LegacyCard({ title = "fallback" }) {
  return <article>{title}</article>;
}

LegacyCard.defaultProps = { title: "fallback" };
LegacyCard.propTypes = { title: PropTypes.string };

const ForwardedInput = forwardRef((props, ref) => <input ref={ref} {...props} />);
const NamespacedForwardedInput = React.forwardRef((props, ref) => <input ref={ref} {...props} />);

ReactDOM.render(<LegacyCard />, rootElement);
ReactDOM.hydrate(<LegacyCard />, rootElement);
ReactDOM.findDOMNode(instance);
ReactDOM.unmountComponentAtNode(rootElement);

class LegacyClass extends React.Component {
  static defaultProps = { title: "supported" };
  static propTypes = { title: PropTypes.string };
  render() {
    return <article>{this.props.title}</article>;
  }
}

// ─── js-react-missing-interval-cleanup ────────────────────────────────────────

// Violation: setInterval inside useEffect without cleanup (triggers js-react-missing-interval-cleanup)
export function PollingComponent() {
  const [data, setData] = useState(null);
  useEffect(() => {
    setInterval(() => {
      fetch("/api/data").then(r => r.json()).then(setData);
    }, 5000);
  }, []);
  return <div>{data}</div>;
}

// Safe: setInterval with cleanup
export function SafePollingComponent() {
  const [data, setData] = useState(null);
  useEffect(() => {
    const id = setInterval(() => {
      fetch("/api/data").then(r => r.json()).then(setData);
    }, 5000);
    return () => clearInterval(id);
  }, []);
  return <div>{data}</div>;
}
