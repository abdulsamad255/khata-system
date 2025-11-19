"use client";

import React, { useEffect, useState } from "react";
import Link from "next/link";

const API_BASE_URL = process.env.NEXT_PUBLIC_API_BASE_URL || "http://localhost:8080";

type Customer = {
  id: number;
  name: string;
  phone: string;
  email: string;
  created_at: string;
};

export default function HomePage() {
  const [customers, setCustomers] = useState<Customer[]>([]);
  const [name, setName] = useState("");
  const [phone, setPhone] = useState("");
  const [email, setEmail] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const fetchCustomers = async () => {
    try {
      setError(null);
      const res = await fetch(`${API_BASE_URL}/api/customers`);
      if (!res.ok) {
        throw new Error("Failed to fetch customers");
      }
      const data: Customer[] = await res.json();
      setCustomers(data);
    } catch (err: any) {
      setError(err.message || "Error loading customers");
    }
  };

  useEffect(() => {
    fetchCustomers();
  }, []);

  const handleCreateCustomer = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!name.trim()) {
      setError("Name is required");
      return;
    }

    setLoading(true);
    setError(null);

    try {
      const res = await fetch(`${API_BASE_URL}/api/customers`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ name, phone, email }),
      });

      if (!res.ok) {
        const body = await res.json().catch(() => null);
        throw new Error(body?.error || "Failed to create customer");
      }

      setName("");
      setPhone("");
      setEmail("");
      await fetchCustomers();
    } catch (err: any) {
      setError(err.message || "Error creating customer");
    } finally {
      setLoading(false);
    }
  };

  return (
    <main className="min-h-screen bg-gray-100">
      <div className="max-w-4xl mx-auto py-10 px-4">
        <h1 className="text-2xl font-bold mb-4">Khata Admin Panel</h1>
        <p className="mb-6 text-gray-700">
          Manage customers and their khata entries (debit/credit).
        </p>

        {/* Create customer form */}
        <section className="mb-10 bg-white shadow p-4 rounded">
          <h2 className="text-xl font-semibold mb-3">Add New Customer</h2>
          {error && (
            <div className="mb-3 text-sm text-red-600">
              {error}
            </div>
          )}
          <form onSubmit={handleCreateCustomer} className="space-y-3">
            <div>
              <label className="block text-sm font-medium mb-1">Name</label>
              <input
                className="border rounded px-3 py-2 w-full"
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="Customer name"
              />
            </div>
            <div>
              <label className="block text-sm font-medium mb-1">Phone</label>
              <input
                className="border rounded px-3 py-2 w-full"
                value={phone}
                onChange={(e) => setPhone(e.target.value)}
                placeholder="Customer phone"
              />
            </div>
            <div>
              <label className="block text-sm font-medium mb-1">Email (optional)</label>
              <input
                className="border rounded px-3 py-2 w-full"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                placeholder="customer@example.com"
              />
            </div>
            <button
              type="submit"
              disabled={loading}
              className="bg-blue-600 text-white px-4 py-2 rounded disabled:opacity-60"
            >
              {loading ? "Saving..." : "Add Customer"}
            </button>
          </form>
        </section>

        {/* Customers list */}
        <section className="bg-white shadow p-4 rounded">
          <h2 className="text-xl font-semibold mb-3">Customers</h2>
          {customers.length === 0 ? (
            <p className="text-gray-600">No customers yet. Add one above.</p>
          ) : (
            <table className="w-full text-sm border">
              <thead className="bg-gray-50">
                <tr>
                  <th className="border px-2 py-1 text-left">Name</th>
                  <th className="border px-2 py-1 text-left">Phone</th>
                  <th className="border px-2 py-1 text-left">Email</th>
                  <th className="border px-2 py-1 text-left">Actions</th>
                </tr>
              </thead>
              <tbody>
                {customers.map((c) => (
                  <tr key={c.id}>
                    <td className="border px-2 py-1">{c.name}</td>
                    <td className="border px-2 py-1">{c.phone}</td>
                    <td className="border px-2 py-1">{c.email}</td>
                    <td className="border px-2 py-1">
                      <Link
                        className="text-blue-600 underline"
                        href={`/customers/${c.id}`}
                      >
                        View Khata
                      </Link>
                    </td>
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
