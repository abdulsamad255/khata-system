"use client";

import React, { useState } from "react";

const API_BASE_URL =
  process.env.NEXT_PUBLIC_API_BASE_URL || "http://localhost:8080";

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

type Totals = {
  debit: number;
  credit: number;
  balance: number;
};

type KhataResult = {
  customer: Customer;
  totals: Totals;
  entries: Entry[];
};

export default function CustomerPortalPage() {
  const [email, setEmail] = useState("");
  const [phone, setPhone] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [result, setResult] = useState<KhataResult | null>(null);

  const handleSearch = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    setResult(null);

    if (!email.trim() || !phone.trim()) {
      setError("Please enter both email and phone.");
      return;
    }

    setLoading(true);

    try {
      const res = await fetch(`${API_BASE_URL}/api/portal/khata-lookup`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ email, phone }),
      });

      if (!res.ok) {
        const body = await res.json().catch(() => null);
        throw new Error(body?.error || "Failed to lookup khata");
      }

      const data: KhataResult = await res.json();
      setResult(data);
    } catch (err: any) {
      setError(err.message || "Something went wrong");
    } finally {
      setLoading(false);
    }
  };

  const handleDownloadPDF = () => {
    if (!result) return;
    const url = `${API_BASE_URL}/api/portal/customers/${result.customer.id}/pdf`;
    // Open in new tab; browser will download the PDF
    window.open(url, "_blank");
  };

  return (
    <main className="min-h-screen bg-gray-100">
      <div className="max-w-3xl mx-auto py-10 px-4">
        <h1 className="text-2xl font-bold mb-2">
          Welcome to the Khata Customer Portal
        </h1>
        <p className="text-gray-700 mb-6">
          Enter your email and phone number to view your khata details.
        </p>

        {/* Search form */}
        <section className="mb-8 bg-white p-4 rounded shadow">
          <h2 className="text-lg font-semibold mb-3">Find My Khata</h2>
          {error && (
            <div className="mb-3 text-sm text-red-600">{error}</div>
          )}
          <form onSubmit={handleSearch} className="space-y-3">
            <div>
              <label className="block text-sm font-medium mb-1">
                Email
              </label>
              <input
                className="border rounded px-3 py-2 w-full"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                placeholder="your-email@example.com"
                type="email"
              />
            </div>
            <div>
              <label className="block text-sm font-medium mb-1">
                Phone
              </label>
              <input
                className="border rounded px-3 py-2 w-full"
                value={phone}
                onChange={(e) => setPhone(e.target.value)}
                placeholder="03XXXXXXXXX"
              />
            </div>
            <button
              type="submit"
              disabled={loading}
              className="bg-blue-600 text-white px-4 py-2 rounded disabled:opacity-60"
            >
              {loading ? "Searching..." : "Search"}
            </button>
          </form>
        </section>

        {/* Result */}
        {result && (
          <section className="bg-white p-4 rounded shadow">
            <div className="flex items-center justify-between mb-3">
              <div>
                <h2 className="text-lg font-semibold">Your Khata</h2>
                <p className="text-sm text-gray-700">
                  Name: {result.customer.name} | Phone:{" "}
                  {result.customer.phone || "N/A"} | Email:{" "}
                  {result.customer.email || "N/A"}
                </p>
              </div>
              <button
                onClick={handleDownloadPDF}
                className="bg-green-600 text-white px-3 py-1 rounded text-sm"
              >
                Download PDF
              </button>
            </div>

            <div className="mb-4 text-sm space-y-1">
              <p>Total Debit: {result.totals.debit.toFixed(2)}</p>
              <p>Total Credit: {result.totals.credit.toFixed(2)}</p>
              <p>
                Balance:{" "}
                <span
                  className={
                    result.totals.balance > 0 ? "text-red-600" : "text-green-600"
                  }
                >
                  {result.totals.balance.toFixed(2)}
                </span>
              </p>
            </div>

            <h3 className="text-md font-semibold mb-2">Entries</h3>
            {result.entries.length === 0 ? (
              <p className="text-sm text-gray-600">
                No entries found on your khata.
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
                  {result.entries.map((e) => (
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
        )}
      </div>
    </main>
  );
}
