// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {ChangeEvent} from 'react';
import React, {useCallback, useRef, useState} from 'react';
import {useIntl} from 'react-intl';

import {Client4} from 'mattermost-redux/client';

import Setting from 'components/widgets/settings/setting';

type Props = {
    id: string;
    label: React.ReactNode;
    helpText?: React.ReactNode;
    accept?: string;
    channelId: string;
    value?: string;
    onChange: (name: string, value: string) => void;
}

const FileUploadSetting = ({
    id,
    label,
    helpText,
    accept,
    channelId,
    value,
    onChange,
}: Props) => {
    const intl = useIntl();
    const inputRef = useRef<HTMLInputElement>(null);
    const [uploading, setUploading] = useState(false);
    const [fileName, setFileName] = useState('');
    const [error, setError] = useState('');
    const [previewUrl, setPreviewUrl] = useState('');

    const handleFileChange = useCallback(async (e: ChangeEvent<HTMLInputElement>) => {
        const file = e.target.files?.[0];
        if (!file) {
            return;
        }

        setError('');
        setUploading(true);
        setFileName(file.name);

        // Show preview for images
        if (file.type.startsWith('image/')) {
            const url = URL.createObjectURL(file);
            setPreviewUrl(url);
        }

        try {
            const formData = new FormData();
            formData.append('files', file);
            formData.append('channel_id', channelId);

            const response = await Client4.uploadFile(formData);
            if (response.file_infos && response.file_infos.length > 0) {
                onChange(id, response.file_infos[0].id);
            }
        } catch {
            setError(intl.formatMessage({
                id: 'interactive_dialog.error.file_upload_failed',
                defaultMessage: 'Failed to upload file. Please try again.',
            }));
            setFileName('');
            setPreviewUrl('');
            onChange(id, '');
        } finally {
            setUploading(false);
        }
    }, [channelId, id, onChange, intl]);

    const handleRemove = useCallback(() => {
        setFileName('');
        setPreviewUrl('');
        setError('');
        onChange(id, '');
        if (inputRef.current) {
            inputRef.current.value = '';
        }
    }, [id, onChange]);

    let helpContent = helpText;
    if (error) {
        helpContent = (
            <>
                {helpText}
                <div className='error-text mt-3'>{error}</div>
            </>
        );
    }

    return (
        <Setting
            label={label}
            helpText={helpContent}
            inputId={id}
        >
            <div>
                {!value && (
                    <input
                        ref={inputRef}
                        id={id}
                        type='file'
                        accept={accept}
                        onChange={handleFileChange}
                        disabled={uploading}
                        className='form-control'
                        style={{padding: '6px'}}
                    />
                )}
                {uploading && (
                    <div className='help-text' style={{marginTop: '8px'}}>
                        {intl.formatMessage({
                            id: 'interactive_dialog.file_uploading',
                            defaultMessage: 'Uploading {fileName}...',
                        }, {fileName})}
                    </div>
                )}
                {value && fileName && !uploading && (
                    <div style={{display: 'flex', alignItems: 'center', gap: '8px', marginTop: '4px'}}>
                        {previewUrl && (
                            <img
                                src={previewUrl}
                                alt={fileName}
                                style={{maxWidth: '80px', maxHeight: '80px', borderRadius: '4px'}}
                            />
                        )}
                        <span>{fileName}</span>
                        <button
                            type='button'
                            className='btn btn-sm btn-tertiary'
                            onClick={handleRemove}
                        >
                            {intl.formatMessage({
                                id: 'interactive_dialog.file_remove',
                                defaultMessage: 'Remove',
                            })}
                        </button>
                    </div>
                )}
            </div>
        </Setting>
    );
};

export default React.memo(FileUploadSetting);
