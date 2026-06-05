"use client";

// Violation 1: async function component
export async function Header() {
  return <header>Header</header>;
}

// Violation 2: async arrow function component
export const Footer = async () => {
  return <footer>Footer</footer>;
};

// Safe 1: sync function component
export function Navbar() {
  return <nav>Navbar</nav>;
}

// Safe 2: async helper function (does not start with uppercase letter)
export async function fetchData() {
  return fetch('/api/data');
}
