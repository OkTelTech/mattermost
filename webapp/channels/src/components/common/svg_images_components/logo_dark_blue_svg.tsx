// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import logoImage from 'images/logo-oktel.png';

type Props = {
    width?: number;
    height?: number;
    className?: string;
}

export default (props: Props) => (
    <img
        className={props.className}
        src={logoImage}
        alt='Oktel'
        width={props.width || 200}
        height={props.height || 200}
        style={{objectFit: 'contain'}}
    />
);
