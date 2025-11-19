"use client";

import React, { useEffect, useState } from "react";
import { useParams, useRouter } from "next/navigation";
import Link from "next/link";

const API_BASE_URL = process.env.NEXT_PUBLIC_API_BASE_URL || "http://localhost:8080";

type Customer = {
  id: number;
  name: string;
  phone: string;
  email: string;
  created_at: string;
};

type Entry = {
  id: number;
  customer_id: number;
  type: "debit" | "credit";
  amount: number;
  note: string;
  created_at: string;
};

export default function CustomerPage() {
  const params = useParams();
  const router = useRouter();
  const id = params?.id as string;

  const [customer, setCustomer] = useState<Customer | null>(null);
  const [entries, setEntries] = useState<Entry[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [entryType, setEntryType] = useState<"debit" | "credit">("debit");
  const [amount, setAmount] = useState("");
  const [note, setNote] = useState("");
  const [saving, setSaving] = useState(false);

  const fetchData = async () => {
    try {
      setError(null);
      setLoading(true);

      // Fetch customer
      const customerRes = await fetch(`${API_BASE_URL}/api/customers/${id}`);
      if (!customerRes.ok) {
        throw new Error("Failed to load customer");
      }
      const customerData: Customer = await customerRes.json();

      // Fetch entries
      const entriesRes = await fetch(`${API_BASE_URL}/api/customers/${id}/entries`);
      if (!entriesRes.ok) {
        throw new Error("Failed to load entries");
      }
      const entriesData: Entry[] = await entriesRes.json();

      setCustomer(customerData);
      setEntries(entriesData);
    } catch (err: any) {
      setError(err.message || "Error loading data");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (id) {
      fetchData();
    }
  }, [id]);

  const totalDebit = entries
    .filter((e) => e.type === "debit")
    .reduce((sum, e) => sum + e.amount, 0);

  const totalCredit = entries
    .filter((e) => e.type === "credit")
    .reduce((sum, e) => sum + e.amount, 0);

  const balance = totalDebit - totalCredit;

  const handleCreateEntry = async (e: React.FormEvent) => {
    e.preventDefault();

    const amt = parseFloat(amount);
    if (isNaN(amt) || amt <= 0) {
      setError("Amount must be a positive number");
      return;
    }

    setSaving(true);
    setError(null);

    try {
      const res = await fetch(`${API_BASE_URL}/api/customers/${id}/entries`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          type: entryType,
          amount: amt,
          note,
        }),
      });

      if (!res.ok) {
        const body = await res.json().catch(() => null);
        throw new Error(body?.error || "Failed to create entry");
      }

      setAmount("");
      setNote("");
      await fetchData();
    } catch (err: any) {
      setError(err.message || "Error creating entry");
    } finally {
      setSaving(false);
    }
  };

  if (loading) {
    return (
      <main className="min-h-screen bg-gray-100">
        <div className="max-w-3xl mx-auto py-10 px-4">Loading...</div>
      </main>
    );
  }

  if (error) {
    return (
      <main className="min-h-screen bg-gray-100">
        <div className="max-w-3xl mx-auto py-10 px-4">
          <p className="mb-4 text-red-600">{error}</p>
          <button
            className="underline text-blue-600"
            onClick={() => router.push("/")}
          >
            Back to customers
          </button>
        </div>
      </main>
    );
  }

  if (!customer) {
    return (
      <main className="min-h-screen bg-gray-100">
        <div className="max-w-3xl mx-auto py-10 px-4">
          <p>Customer not found.</p>
          <button
            className="underline text-blue-600"
            onClick={() => router.push("/")}
          >
            Back to customers
          </button>
        </div>
      </main>
    );
  }

  return (
    <main className="min-h-screen bg-gray-100">
      <div className="max-w-3xl mx-auto py-10 px-4">
        <div className="mb-4">
          <Link href="/" className="text-blue-600 underline">
            ← Back to customers
          </Link>
        </div>

        <h1 className="text-2xl font-bold mb-1">Khata for {customer.name}</h1>
        <p className="text-gray-700 mb-4">
          Phone: {customer.phone || "N/A"} | Email: {customer.email || "N/A"}
        </p>

        {/* Summary */}
        <section className="mb-8 bg-white p-4 rounded shadow">
          <h2 className="text-lg font-semibold mb-2">Summary</h2>
          <div className="space-y-1 text-sm">
            <p>Total Debit: {totalDebit.toFixed(2)}</p>
            <p>Total Credit: {totalCredit.toFixed(2)}</p>
            <p>
              Balance:{" "}
              <span className={balance > 0 ? "text-red-600" : "text-green-600"}>
                {balance.toFixed(2)}
              </span>
            </p>
          </div>
        </section>

        {/* New entry form */}
        <section className="mb-8 bg-white p-4 rounded shadow">
          <h2 className="text-lg font-semibold mb-2">Add Entry</h2>
          {error && (
            <div className="mb-2 text-sm text-red-600">
              {error}
            </div>
          )}
          <form onSubmit={handleCreateEntry} className="space-y-3">
            <div>
              <label className="block text-sm font-medium mb-1">Type</label>
              <select
                className="border rounded px-3 py-2 w-full"
                value={entryType}
                onChange={(e) => setEntryType(e.target.value as "debit" | "credit")}
              >
                <option value="debit">Debit (Customer owes you)</option>
                <option value="credit">Credit (Customer paid you)</option>
              </select>
            </div>
            <div>
              <label className="block text-sm font-medium mb-1">Amount</label>
              <input
                className="border rounded px-3 py-2 w-full"
                value={amount}
                onChange={(e) => setAmount(e.target.value)}
                placeholder="1000"
              />
            </div>
            <div>
              <label className="block text-sm font-medium mb-1">Note</label>
              <input
                className="border rounded px-3 py-2 w-full"
                value={note}
                onChange={(e) => setNote(e.target.value)}
                placeholder="e.g. Grocery, partial payment"
              />
            </div>
            <button
              type="submit"
              disabled={saving}
              className="bg-blue-600 text-white px-4 py-2 rounded disabled:opacity-60"
            >
              {saving ? "Saving..." : "Add Entry"}
            </button>
          </form>
        </section>

        {/* Entries list */}
        <section className="bg-white p-4 rounded shadow">
          <h2 className="text-lg font-semibold mb-2">Entries</h2>
          {entries.length === 0 ? (
            <p className="text-sm text-gray-600">
              No entries yet. Add a debit or credit above.
            </p>
          ) : (
            <table className="w-full text-sm border">
              <thead className="bg-gray-50">
                <tr>
                  <th className="border px-2 py-1 text-left">Date</th>
                  <th className="border px-2 py-1 text-left">Type</th>
                  <th className="border px-2 py-1 text-right">Amount</th>
                  <th className="border px-2 py-1 text-left">Note</th>
                </tr>
              </thead>
              <tbody>
                {entries.map((e) => (
                  <tr key={e.id}>
                    <td className="border px-2 py-1">
                      {new Date(e.created_at).toLocaleString()}
                    </td>
                    <td className="border px-2 py-1">
                      {e.type === "debit" ? "Debit" : "Credit"}
                    </td>
                    <td className="border px-2 py-1 text-right">
                      {e.amount.toFixed(2)}
                    </td>
                    <td className="border px-2 py-1">{e.note}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </section>
      </div>
    </main>
  );
}
