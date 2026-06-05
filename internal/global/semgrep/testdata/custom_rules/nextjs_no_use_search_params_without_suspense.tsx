import { useSearchParams } from 'next/navigation';

export function SearchComponent() {
  // Violation: call useSearchParams()
  const searchParams = useSearchParams();
  const query = searchParams.get('q');
  return <div>Query: {query}</div>;
}
