import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { BrowserRouter, Route, Routes } from "react-router-dom";
import Dashboard from "./pages/Dashboard";
import SampleDetail from "./pages/SampleDetail";

const queryClient = new QueryClient();

export default function App() {
	return (
		<QueryClientProvider client={queryClient}>
			<BrowserRouter>
				<div className="min-h-screen bg-slate-50 text-slate-900">
					<div className="mx-auto max-w-[960px] px-6 pt-8 pb-16">
						<Routes>
							<Route path="/" element={<Dashboard />} />
							<Route path="/samples/:jobId" element={<SampleDetail />} />
						</Routes>
					</div>
				</div>
			</BrowserRouter>
		</QueryClientProvider>
	);
}
