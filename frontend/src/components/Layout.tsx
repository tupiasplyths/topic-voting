import { Link, useLocation } from 'react-router-dom';
import { LayoutDashboard, Monitor, MessageSquare } from 'lucide-react';

export default function Layout({ children }: { children: React.ReactNode }) {
  const location = useLocation();

  const navItem = (path: string, icon: React.ReactNode, label: string) => {
    const active = location.pathname === path;
    return (
      <Link
        to={path}
        className={`flex items-center gap-2 px-3 py-2 rounded-lg text-sm font-medium transition-colors ${
          active
            ? 'bg-indigo-600 text-white'
            : 'text-gray-300 hover:bg-gray-700 hover:text-white'
        }`}
      >
        {icon}
        {label}
      </Link>
    );
  };

  return (
    <div className="min-h-screen bg-gray-900 text-gray-100">
      <nav className="border-b border-gray-700 bg-gray-800">
        <div className="max-w-7xl mx-auto px-4 py-3 flex items-center gap-4">
          <h1 className="text-lg font-bold text-white mr-4">Topic Voting</h1>
          {navItem('/', <LayoutDashboard size={16} />, 'Host')}
          {navItem('/display', <Monitor size={16} />, 'Display')}
          {navItem('/chat', <MessageSquare size={16} />, 'Chat')}
        </div>
      </nav>
      <main className="max-w-7xl mx-auto px-4 py-6">{children}</main>
    </div>
  );
}
