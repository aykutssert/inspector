// @ts-nocheck
import React, { forwardRef } from "react";
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
