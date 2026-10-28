import React from 'react';
import type { ServerViewModel } from '@pages/dashboard/viewModel';
import FlipCard from '@components/dashboard/FlipCard';
import ServerCardBack from './ServerCardBack';
import ServerCardFront from './ServerCardFront';

interface Props {
  view: ServerViewModel;
  canOpenHistory: boolean;
  canOpenTraffic: boolean;
  canOpenPVE: boolean;
}

const ServerCard: React.FC<Props> = ({ view, canOpenHistory, canOpenTraffic, canOpenPVE }) => {
  return (
    <div className="[content-visibility:auto] [contain-intrinsic-block-size:428px]">
      <FlipCard
        disabled={!view.isAlive}
        front={
          <ServerCardFront
            view={view}
            canOpenHistory={canOpenHistory}
            canOpenTraffic={canOpenTraffic}
            canOpenPVE={canOpenPVE}
          />
        }
        back={<ServerCardBack view={view} />}
      />
    </div>
  );
};

export default ServerCard;
