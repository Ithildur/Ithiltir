import React from 'react';
import type { ServerViewModel } from '@pages/dashboard/viewModel';
import FlipCard from '@components/dashboard/FlipCard';
import ServerCardBack from './ServerCardBack';
import ServerCardFront from './ServerCardFront';

interface Props {
  view: ServerViewModel;
  canOpenHistory: boolean;
  canOpenTraffic: boolean;
}

const ServerCard: React.FC<Props> = ({ view, canOpenHistory, canOpenTraffic }) => {
  return (
    <div className="[content-visibility:auto] [contain-intrinsic-block-size:408px]">
      <FlipCard
        disabled={!view.isAlive}
        front={
          <ServerCardFront
            view={view}
            canOpenHistory={canOpenHistory}
            canOpenTraffic={canOpenTraffic}
          />
        }
        back={<ServerCardBack view={view} />}
      />
    </div>
  );
};

export default ServerCard;
