import { BrowserRouter, Routes, Route } from 'react-router-dom';
import Layout from './components/Layout';
import HostPage from './pages/HostPage';
import DisplayPage from './pages/DisplayPage';
import ChatPage from './pages/ChatPage';

export default function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route
          path="/"
          element={
            <Layout>
              <HostPage />
            </Layout>
          }
        />
        <Route path="/display" element={<DisplayPage />} />
        <Route
          path="/chat"
          element={
            <Layout>
              <ChatPage />
            </Layout>
          }
        />
      </Routes>
    </BrowserRouter>
  );
}
