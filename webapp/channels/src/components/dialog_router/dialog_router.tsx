// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import InteractiveDialog from 'components/interactive_dialog/interactive_dialog';

import InteractiveDialogAdapter from './interactive_dialog_adapter';

import type {PropsFromRedux} from './index';

// Make props optional like the original InteractiveDialog, but keep required props
type OptionalPropsFromRedux = Partial<PropsFromRedux> & Pick<PropsFromRedux, 'emojiMap' | 'isAppsFormEnabled' | 'hasUrl' | 'actions'>;

type Props = OptionalPropsFromRedux & {
    onExited?: () => void;
};

const DialogRouter: React.FC<Props> = (props) => {
    const {isAppsFormEnabled, hasUrl, elements} = props;

    // URL-less dialog = configuration error
    if (!hasUrl) {
        // eslint-disable-next-line no-console
        console.error('Interactive dialog missing URL - this is a configuration error');
        return null; // Let calling code show ephemeral error
    }

    // Use legacy dialog for custom element types (e.g. file upload) not supported by Apps adapter
    const hasCustomElements = elements?.some((e) => e.type === 'file');

    if (isAppsFormEnabled && !hasCustomElements) {
        return <InteractiveDialogAdapter {...props}/>;
    }

    return <InteractiveDialog {...props}/>;
};

export default DialogRouter;
