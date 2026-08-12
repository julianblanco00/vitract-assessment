import { BrowserRouter, Route, Routes } from "react-router-dom";
import Dashboard from "./pages/Dashboard";
import SampleDetail from "./pages/SampleDetail";

export default function App() {
  return (
    <BrowserRouter>
      <div className="app-shell">
        <Routes>
          <Route path="/" element={<Dashboard />} />
          <Route path="/samples/:jobId" element={<SampleDetail />} />
        </Routes>
      </div>
    </BrowserRouter>
  );
}
