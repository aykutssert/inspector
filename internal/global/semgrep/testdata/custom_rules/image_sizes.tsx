// @ts-nocheck
import Image from "next/image";

export function ImageComponent() {
  return (
    <div>
      {/* Violation: nextjs-image-missing-sizes */}
      <Image src="/test.jpg" fill alt="Test" />

      {/* Violation: nextjs-image-missing-sizes */}
      <Image src="/test.jpg" fill={true} alt="Test" />

      {/* Violation: nextjs-image-missing-sizes */}
      <Image src="/test.jpg" layout="fill" alt="Test" />

      {/* Violation: nextjs-image-missing-sizes */}
      <Image src="/test.jpg" layout="responsive" alt="Test" />

      {/* Safe: has sizes */}
      <Image src="/test.jpg" fill sizes="(max-width: 768px) 100vw, 50vw" alt="Test" />

      {/* Safe: has sizes as expression */}
      <Image src="/test.jpg" fill sizes={someSizesVar} alt="Test" />

      {/* Safe: static width/height (no fill or responsive layout) */}
      <Image src="/test.jpg" width={100} height={100} alt="Test" />
    </div>
  );
}
