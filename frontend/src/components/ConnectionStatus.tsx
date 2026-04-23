import { Wifi, WifiOff, AlertCircle } from 'lucide-react';

type WSStatus = 'connecting' | 'open' | 'closed' | 'error';

interface Props {
  wsStatus: WSStatus;
}

export default function ConnectionStatus({ wsStatus }: Props) {
  const config: Record<WSStatus, { icon: React.ReactNode; label: string; color: string }> = {
    open: {
      icon: <Wifi size={14} />,
      label: 'Connected',
      color: 'text-green-400',
    },
    connecting: {
      icon: <AlertCircle size={14} />,
      label: 'Connecting...',
      color: 'text-yellow-400',
    },
    closed: {
      icon: <WifiOff size={14} />,
      label: 'Disconnected',
      color: 'text-red-400',
    },
    error: {
      icon: <WifiOff size={14} />,
      label: 'Connection Error',
      color: 'text-red-400',
    },
  };

  const { icon, label, color } = config[wsStatus];

  return (
    <div className={`flex items-center gap-1.5 text-xs ${color}`}>
      {icon}
      <span>{label}</span>
    </div>
  );
}
