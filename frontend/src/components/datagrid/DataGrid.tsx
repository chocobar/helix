import '@inovua/reactdatagrid-community/index.css'
import React, { FC, useCallback } from 'react'
import { useTheme } from '@mui/material/styles'
import ReactDataGrid from '@inovua/reactdatagrid-community'
import Box from '@mui/material/Box'
import { SxProps } from '@mui/system'
import Loading from '../system/Loading'
import useThemeConfig from '../../hooks/useThemeConfig'

export interface IDataGrid2_Column_Render_Params<DataType = any> {
  value: any,
  data: DataType,
}

export interface IDataGrid2_Column<DataType = any> {
  name: string,
  header: string,
  defaultFlex?: number,
  defaultWidth?: number,
  minWidth?: number,
  textAlign?: 'start' | 'center' | 'end',
  cellStyle?: any,
  render?: (params: IDataGrid2_Column_Render_Params<DataType>) => any, 
}

interface DataGridProps<DataType = any> {
  idProperty?: string,
  rows: DataType[],
  columns: IDataGrid2_Column<DataType>[],
  sx?: SxProps,
  innerSx?: SxProps,
  loading?: boolean,
  userSelect?: boolean,
  minHeight?: number,
  rowHeight?: number,
  headerHeight?: number,
  autoSort?: boolean,
  editable?: boolean,
  onSelect?: {
    (rowIdx: number, colIdx: number): void,
  },
  onDoubleClick?: {
    (rowIdx: number): void,
  },
}

const DataGrid: FC<React.PropsWithChildren<DataGridProps>> = ({
  idProperty = 'id',
  rows,
  columns,
  sx={},
  innerSx = {},
  userSelect = false,
  minHeight = 400,
  editable = false,
  rowHeight = 56,
  headerHeight = 40,
  loading,
  onDoubleClick,
  onSelect,
}) => {
  const theme = useTheme()
  const themeConfig = useThemeConfig()
  const onCellClick = useCallback((ev: any, cellProps: any) => {
    if(!onSelect) return
    onSelect(cellProps.rowIndex, cellProps.columnIndex)
  }, [onSelect])

  const borderStyle = `1px solid ${theme.palette.mode === 'light' ? themeConfig.lightBorder : themeConfig.darkBorder}`

  const gridStyle = {
    backgroundColor: theme.palette.mode === 'light' ? themeConfig.lightBackgroundColor : themeConfig.darkBackgroundColor,
    minHeight: 400,
    boxShadow: '0px 4px 10px 0px rgba(0,0,0,0.1)',
    width: '100%',
    height: '100%',
    position: 'relative',
    flexGrow: 1,
    flexBasis: '100%',
    border: borderStyle,
  }

  return (
    <Box
      className='grid-wrapper'
      sx={{
        width: '100%',
        height: '100%',
        minHeight: '100%',
        position: 'relative',
        display: 'flex',
        overflow: 'auto',
        backgroundColor: 'transparent',
        boxShadow: '0 2px 4px 0px rgba(0,0,0,0.2)',
        border: borderStyle,
        // TODO: for the love of god please can we sort this out
        // all we want is some border please
        '& .InovuaReactDataGrid--theme-default-light': {
          border: '0.1rem solid #303047',
        },
        '& .InovuaReactDataGrid--theme-default-light .InovuaReactDataGrid__column-header--show-border-left, .InovuaReactDataGrid--theme-default-light .InovuaReactDataGrid__column-header__resize-wrapper--show-border-left, .InovuaReactDataGrid--theme-default-light .InovuaReactDataGrid__header-group__title--show-border-left': {
          borderLeft: '0.1rem solid #303047 !important',
        },
        '& .InovuaReactDataGrid--theme-default-light .InovuaReactDataGrid__header': {
          borderBottom: '0.1rem solid #303047 !important',
        },
        '& .InovuaReactDataGrid--theme-default-light .InovuaReactDataGrid__cell--show-border-bottom': {
          borderBottom: '0.1rem solid #303047 !important',
        },
        '& .InovuaReactDataGrid__header': {
          color: theme.palette.mode === 'dark' ? theme.palette.grey[300] : theme.palette.grey[900],
          fontWeight: 'lighter',
          backgroundColor: theme.palette.mode === 'light' ? themeConfig.lightBackgroundColor : themeConfig.darkBackgroundColor,
          borderTop: borderStyle,
          borderBottom: borderStyle,
        },
        '& .InovuaReactDataGrid__row': {
          backgroundColor: theme.palette.mode === 'light' ? themeConfig.lightBackgroundColor : themeConfig.darkBackgroundColor,
          color: theme.palette.mode === 'dark' ? theme.palette.grey[300] : theme.palette.grey[900],
          borderTop: borderStyle,
          borderBottom: borderStyle,
          '&:hover': {
            backgroundColor: theme.palette.mode === 'light' ? themeConfig.lightBackgroundColor : themeConfig.darkBackgroundColor,
          },
        },
        ...sx,
      }}
    >
      <Box
        className='Grid'
        sx={{
          display: 'flex',
          flexDirection: 'column',
          height: '100%',
          flexGrow: 1,
          '& .rdg': {
            userSelect: userSelect ? 'auto' : 'none'
          },
          minHeight: `${minHeight}px`,
          ...innerSx,
        }}
      >
        <ReactDataGrid
          columnUserSelect={!editable}
          editable={editable}
          idProperty={idProperty}
          columns={columns}
          dataSource={rows}
          onCellClick={onCellClick}
          onCellDoubleClick={(ev, props) => onDoubleClick && onDoubleClick(props.rowIndex)}
          headerHeight={headerHeight}
          minRowHeight={rowHeight}
          rowHeight={null}
          style={gridStyle}
          showCellBorders={false}
          showHoverRows={false}
        />
      </Box>
      {loading && (
        <Box
          sx={{
            position: 'absolute',
            width: '100%',
            height: '100%',
            left: '0px',
            top: '0px',
            backgroundColor: 'rgba(255,255,255,0.8)',
          }}
        >
          <Loading />
        </Box>
      )}
    </Box>
  )
}

export default DataGrid
