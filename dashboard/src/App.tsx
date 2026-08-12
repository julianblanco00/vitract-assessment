import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { BrowserRouter, Route, Routes } from "react-router-dom";
import Dashboard from "./pages/Dashboard";
import SampleDetail from "./pages/SampleDetail";

const queryClient = new QueryClient();

export default function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <div className="app-shell">
          <Routes>
            <Route path="/" element={<Dashboard />} />
            <Route path="/samples/:jobId" element={<SampleDetail />} />
          </Routes>
        </div>
      </BrowserRouter>
    </QueryClientProvider>
  );
}
