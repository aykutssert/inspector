export default function Page() {
  return (
    <div>
      {/* trigger design-no-space-on-flex-children */}
      <div className="flex space-x-4">
        <span>A</span>
      </div>

      {/* trigger design-no-space-on-flex-children */}
      <div className={"flex space-y-2"}>
        <span>B</span>
      </div>

      {/* safe: using gap */}
      <div className="flex gap-4">
        <span>C</span>
      </div>
    </div>
  );
}
