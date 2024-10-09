import PropTypes from 'prop-types';
import { useState } from 'react';

import { CHANNEL_OPTIONS } from 'constants/ChannelConstants';

import {
  Popover,
  TableRow,
  MenuItem,
  TableCell,
  IconButton,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
  Button,
  Stack,
  Collapse
} from '@mui/material';
import { useTheme } from '@mui/material/styles';

import Label from 'ui-component/Label';
import ModelsPopover from 'ui-component/ModelsPopover';
import KeyboardArrowDownIcon from '@mui/icons-material/KeyboardArrowDown';
import KeyboardArrowUpIcon from '@mui/icons-material/KeyboardArrowUp';

import GroupLabel from './GroupLabel';
import ChannelTable from './ChannelTable';
import { IconDotsVertical, IconEdit, IconTrash } from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';

export default function TagTableRow({ item, manageChannel, handleOpenModal, setModalChannelId }) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(null);
  const [openDelete, setOpenDelete] = useState(false);
  const [openRow, setOpenRow] = useState(false);
  const theme = useTheme();

  let modelMap = [];
  modelMap = item.models.split(',');
  modelMap.sort();

  const handleDeleteOpen = () => {
    handleCloseMenu();
    setOpenDelete(true);
  };

  const handleDeleteClose = () => {
    setOpenDelete(false);
  };

  const handleOpenMenu = (event) => {
    setOpen(event.currentTarget);
  };

  const handleCloseMenu = () => {
    setOpen(null);
  };

  const handleDelete = async () => {
    handleCloseMenu();
    await manageChannel(item.tag, 'delete', '');
  };
  return (
    <>
      <TableRow tabIndex={item.tag}>
        <TableCell>
          <IconButton aria-label="expand row" size="small" onClick={() => setOpenRow(!openRow)}>
            {openRow ? <KeyboardArrowUpIcon /> : <KeyboardArrowDownIcon />}
          </IconButton>
        </TableCell>

        <TableCell>{item.tag}</TableCell>
        <TableCell>
          {!CHANNEL_OPTIONS[item.type] ? (
            <Label color="error" variant="outlined">
              {t('common.unknown')}
            </Label>
          ) : (
            <Label color={CHANNEL_OPTIONS[item.type].color} variant="outlined">
              {CHANNEL_OPTIONS[item.type].text}
            </Label>
          )}
        </TableCell>

        <TableCell>
          <GroupLabel group={item.group} />
        </TableCell>

        <TableCell>
          <ModelsPopover model={item.models} />
        </TableCell>

        <TableCell>
          <Stack direction="row" justifyContent="center" alignItems="center" spacing={1}>
            <IconButton onClick={handleOpenMenu} sx={{ color: 'rgb(99, 115, 129)' }}>
              <IconDotsVertical />
            </IconButton>
          </Stack>
        </TableCell>
      </TableRow>

      <TableRow
        sx={{
          backgroundColor: theme.components.MuiTableCell.styleOverrides.head.backgroundColor
        }}
      >
        <TableCell
          style={{
            paddingBottom: 0,
            paddingTop: 0,
            textAlign: 'left'
          }}
          colSpan={20}
        >
          <Collapse in={openRow} timeout="auto" unmountOnExit>
            <ChannelTable tag={item.tag} />
          </Collapse>
        </TableCell>
      </TableRow>

      <Popover
        open={!!open}
        anchorEl={open}
        onClose={handleCloseMenu}
        anchorOrigin={{ vertical: 'top', horizontal: 'left' }}
        transformOrigin={{ vertical: 'top', horizontal: 'right' }}
        PaperProps={{
          sx: { minWidth: 140 }
        }}
      >
        <MenuItem
          onClick={() => {
            handleCloseMenu();
            handleOpenModal();
            setModalChannelId(item.tag);
          }}
        >
          <IconEdit style={{ marginRight: '16px' }} />
          {t('common.edit')}
        </MenuItem>

        <MenuItem onClick={handleDeleteOpen} sx={{ color: 'error.main' }}>
          <IconTrash style={{ marginRight: '16px' }} />
          {t('common.delete')}
        </MenuItem>
      </Popover>
      <Dialog open={openDelete} onClose={handleDeleteClose}>
        <DialogTitle>{t('channel_row.delTag')}</DialogTitle>
        <DialogContent>
          <DialogContentText>
            {t('channel_row.delTagInfo1')}
            {item.name}？<br /> {t('channel_row.delTagInfo2')}
          </DialogContentText>
        </DialogContent>
        <DialogActions>
          <Button onClick={handleDeleteClose}>{t('common.close')}</Button>
          <Button onClick={handleDelete} sx={{ color: 'error.main' }} autoFocus>
            {t('common.delete')}
          </Button>
        </DialogActions>
      </Dialog>
    </>
  );
}

TagTableRow.propTypes = {
  item: PropTypes.object,
  manageChannel: PropTypes.func,
  handleOpenModal: PropTypes.func,
  setModalChannelId: PropTypes.func
};
