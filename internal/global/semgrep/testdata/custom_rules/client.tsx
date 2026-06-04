// @ts-nocheck
"use client";

import { useState } from "react";
import fs from "fs"; // should trigger next-server-only-in-client
import { Example } from "./case";

import { useMutation, useQueryClient } from "@tanstack/react-query";

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

