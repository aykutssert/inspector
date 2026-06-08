// @ts-nocheck
"use client";

import { startTransition, useState } from "react";
import { flushSync } from "react-dom";
import fs from "fs"; // should trigger next-server-only-in-client
import { Example } from "./case";

import { useMutation, useQueryClient, useQuery, useSuspenseQuery } from "@tanstack/react-query";

export function ClientComponent() {
  const [state, setState] = useState(0);
  const data = fs.readFileSync("test.txt", "utf8"); // should trigger next-server-only-in-client (via import)
  const apiToken = process.env.VITE_API_TOKEN; // triggers vite-process-env-usage
  const nodeEnv = process.env.NODE_ENV; // ok
  return <div>{state}</div>;
}

export function MutationTestComponent() {
  const queryClient = useQueryClient();

  // Violations (missing invalidation/update)
  const mutation1 = useMutation({
    mutationFn: (newTodo) => axios.post('/todos', newTodo),
  }); // triggers tanstack-mutation-without-invalidation

  const mutation2 = useMutation({
    mutationFn: (newTodo) => axios.post('/todos', newTodo),
    onSuccess: () => {
      console.log("Success but no invalidation");
    }
  }); // triggers tanstack-mutation-without-invalidation

  const mutation3 = useMutation({
    mutationFn: (newTodo) => axios.post('/todos', newTodo),
    onSettled: function() {
      console.log("Settled but no invalidation");
    }
  }); // triggers tanstack-mutation-without-invalidation

  // Safe cases (with invalidation/update)
  const safeMutation1 = useMutation({
    mutationFn: (newTodo) => axios.post('/todos', newTodo),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['todos'] });
    }
  });

  const safeMutation2 = useMutation({
    mutationFn: (newTodo) => axios.post('/todos', newTodo),
    onSuccess: () => queryClient.setQueryData(['todos'], (old) => [...old, newTodo]),
  });

  const safeMutation3 = useMutation({
    mutationFn: (newTodo) => axios.post('/todos', newTodo),
    onSettled: function() {
      invalidateQueries(['todos']);
    }
  });
}

export function QueryTestComponent() {
  // Violations (missing staleTime)
  const query1 = useQuery({
    queryKey: ['todos'],
    queryFn: () => axios.get('/todos'),
  }); // triggers tanstack-query-missing-staletime

  const query2 = useSuspenseQuery({
    queryKey: ['projects'],
    queryFn: () => axios.get('/projects'),
  }); // triggers tanstack-query-missing-staletime

  // Safe cases (with staleTime)
  const safeQuery1 = useQuery({
    queryKey: ['todos'],
    queryFn: () => axios.get('/todos'),
    staleTime: 5000,
  });

  const safeQuery2 = useSuspenseQuery({
    queryKey: ['projects'],
    queryFn: () => axios.get('/projects'),
    staleTime: 1000 * 60 * 5,
  });

  // Older signature safe cases
  const safeQuery3 = useQuery(['todos'], () => axios.get('/todos'), {
    staleTime: 10000,
  });
}

export function StorageVersioningTest() {
  // Violations: unversioned keys (triggers general.client-localstorage-no-version)
  localStorage.setItem("user", "john");
  localStorage.getItem("token");
  sessionStorage.setItem("settings", "{}");
  sessionStorage.getItem("cart");

  // Safe: versioned keys or dynamic keys
  localStorage.setItem("user:v1", "john");
  localStorage.getItem("token_v2");
  sessionStorage.setItem("settings:version1", "{}");
  sessionStorage.getItem("cart:V3");
  
  const dynamicKey = "user";
  localStorage.setItem(dynamicKey, "john");
}

export function PassiveEventListenersTest(element: HTMLElement, handleTouch: any, handleWheel: any) {
  // Violations: unregistered options/non-passive (triggers general.client-passive-event-listeners)
  window.addEventListener("touchstart", (e) => { console.log("touch"); });
  document.addEventListener("touchmove", handleTouch);
  addEventListener("wheel", (e) => { console.log("wheel"); });
  element.addEventListener("mousewheel", handleWheel, { capture: true });
  element.addEventListener("touchstart", handleTouch, false);

  // Safe: calls preventDefault, uses passive: true, or different event types
  window.addEventListener("touchstart", (e) => { e.preventDefault(); });
  document.addEventListener("touchmove", (e) => e.preventDefault());
  addEventListener("wheel", (e) => { console.log(e); }, { passive: true });
  element.addEventListener("touchstart", handleTouch, { passive: true, capture: true });
  element.addEventListener("click", (e) => { console.log("click"); });
}

export function ViewportTest() {
  return (
    <div>
      {/* Violations: disabled zoom (triggers general.no-disabled-zoom) */}
      <meta name="viewport" content="width=device-width, user-scalable=no" />
      <meta name="viewport" content="width=device-width, initial-scale=1, maximum-scale=1" />
      <meta name="viewport" content="width=device-width, initial-scale=1, maximum-scale=1.0" />
      <meta name="viewport" content="width=device-width, initial-scale=1, user-scalable=0" />

      {/* Safe: zoom enabled */}
      <meta name="viewport" content="width=device-width, initial-scale=1" />
      <meta name="viewport" content="width=device-width, initial-scale=1, user-scalable=yes" />
      <meta name="viewport" content="width=device-width, initial-scale=1, maximum-scale=5" />
    </div>
  );
}

export function RuleWaveSevenTest() {
  const [open, setOpen] = useState(false);

  const forceSync = () => {
    flushSync(() => {
      setOpen(true);
    });
  };

  const safeAsync = () => {
    startTransition(() => {
      setOpen(false);
    });
  };

  return (
    <section>
      <form onSubmit={(event) => {
        event.preventDefault();
        setOpen(true);
      }}>
        <button type="submit">Save</button>
      </form>
      <a href="/settings" onClick={(event) => event.preventDefault()}>Settings</a>

      <div style={{ transition: "width 200ms ease" }} />
      <div style={{ transitionProperty: "top, opacity" }} />
      <div style={{ animation: "height-pulse 200ms ease-in-out" }} />
      <div style={{ transition: "transform 200ms ease", opacity: open ? 1 : 0.5 }} />

      <button style={{ outline: "none" }}>Outline removed</button>
      <button style={{ outlineWidth: 0 }}>Outline width zero</button>
      <button style={{ boxShadow: "0 0 0 2px currentColor" }}>Custom focus ring</button>

      <p style={{ fontSize: "10px" }}>Tiny copy</p>
      <span className="text-[11px]">Tiny utility text</span>
      <span style={{ fontSize: "12px" }}>Readable copy</span>
      <span className="text-[12px]">Readable utility text</span>

      <button type="button" onClick={forceSync}>Force sync</button>
      <button type="button" onClick={safeAsync}>Transition</button>
    </section>
  );
}


