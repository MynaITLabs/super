import React, { useState } from 'react'
import PropTypes from 'prop-types'
import { Icon } from 'FontAwesomeUtils'
import {
  faAddressCard,
  faArrowRight,
  faArrowRightLong,
  faBan,
  faBroadcastTower,
  faCircleInfo,
  faCirclePlus,
  faClock,
  faEllipsis,
  faPlus,
  faTag,
  faTags,
  faXmark
} from '@fortawesome/free-solid-svg-icons'

import {
  Box,
  Button,
  IconButton,
  FormControl,
  Input,
  HStack,
  VStack,
  Menu,
  Popover,
  Text,
  Tooltip,
  useColorModeValue
} from 'native-base'
import { isMetaProperty, isTemplateSpan } from 'typescript'

const FlowCard = ({ icon, title, body, description, size, edit, ...props }) => {
  size = size || 'md'

  const trigger = (triggerProps) => (
    <IconButton
      variant="unstyled"
      ml="auto"
      icon={<Icon icon={faEllipsis} color="muted.600" />}
      {...triggerProps}
    ></IconButton>
  )

  const onDelete = props.onDelete || function () {}

  const moreMenu = (
    <Menu w="190" closeOnSelect={true} trigger={trigger}>
      {/*<Menu.Item>Edit</Menu.Item>*/}
      <Menu.Item _text={{ color: 'danger.600' }} onPress={onDelete}>
        Delete
      </Menu.Item>
    </Menu>
  )

  return (
    <Box
      bg={useColorModeValue('muted.50', 'blueGray.700')}
      p={size == 'xs' ? 2 : 4}
      borderRadius={5}
      shadow={5}
      rounded="md"
      minW={340}
      maxWidth="100%"
      {...props}
    >
      <HStack justifyContent="stretch" alignItems="center" space={4}>
        <Box
          height={size == 'xs' ? 30 : 50}
          rounded="full"
          width={size == 'xs' ? 30 : 50}
          justifyContent="center"
          alignItems="center"
        >
          {icon}
        </Box>
        <VStack alignContent="center" space={size == 'xs' ? 0 : 1}>
          <HStack space={1} alignItems="center">
            <Text color="muted.400" fontSize="sm">
              {title}
            </Text>
            {description ? (
              <Tooltip
                label={description}
                bg="muted.800"
                _text={{ color: 'muted.200' }}
              >
                <IconButton
                  variant="unstyled"
                  icon={
                    <Icon icon={faCircleInfo} size="xs" color="muted.200" />
                  }
                />
              </Tooltip>
            ) : null}
          </HStack>
          <Text>{body}</Text>
        </VStack>
        {edit ? moreMenu : null}
      </HStack>
    </Box>
  )
}

FlowCard.propTypes = {
  title: PropTypes.string.isRequired,
  body: PropTypes.oneOfType([
    PropTypes.string.isRequired,
    PropTypes.element.isRequired
  ]),
  icon: PropTypes.element.isRequired,
  description: PropTypes.string,
  size: PropTypes.string,
  edit: PropTypes.bool
}

// token is like variables but for cards
// TODO use proptypes to describe the cards
const Token = ({ value: defaultValue, label, onChange, ...props }) => {
  const [value, setValue] = useState(defaultValue)

  const trigger = (triggerProps) => (
    <Button
      variant="outline"
      colorScheme="light"
      rounded="md"
      size="sm"
      p={1}
      lineHeight={14}
      textAlign="center"
      {...triggerProps}
    >
      {value}
    </Button>
  )

  return (
    <>
      {label ? <Text mr={1}>{label}</Text> : null}
      <Popover trigger={trigger}>
        <Popover.Content>
          <Popover.Body>
            <HStack space={1}>
              <FormControl flex={1}>
                <Input
                  variant="outlined"
                  defaultValue={value}
                  onChangeText={(value) => setValue(value)}
                />
              </FormControl>
              <IconButton
                ml="auto"
                colorScheme="light"
                icon={<Icon icon={faTag} />}
              />
            </HStack>
          </Popover.Body>
        </Popover.Content>
      </Popover>
    </>
  )
}

const TriggerCardDate = ({ item, edit, ...props }) => {
  return (
    <FlowCard
      title={item.title}
      body={
        edit ? (
          <HStack space={1} justifyContent="space-around" alignItems="center">
            <Token
              value={item.props.days.join(',')}
              onChange={(value) => {
                item.days = value.split(',')
              }}
            />
            <Token
              value={item.props.from}
              onChange={(value) => {
                item.from = value
              }}
            />
            <Text>-</Text>
            <Token
              value={item.props.to}
              onChange={(value) => {
                item.to = value
              }}
            />
          </HStack>
        ) : (
          <HStack space={1}>
            <Text>Weekdays</Text>
            <Text>{item.props.from}</Text>
            <Text>-</Text>
            <Text>{item.props.to}</Text>
          </HStack>
        )
      }
      icon={
        <Icon
          icon={faClock}
          color="violet.300"
          size={props.size == 'xs' ? '8x' : '12x'}
        />
      }
      {...props}
    />
  )
}

const ActionCardBlock = ({ item, edit, ...props }) => (
  <FlowCard
    title={`Block ${item.Protocol.toUpperCase()}`}
    body={
      edit ? (
        <HStack space={1}>
          <Token label="Source" value={item.SrcIP} />
          <Token label="Dest" value={item.DstIP} />
        </HStack>
      ) : (
        <HStack space={1}>
          <Text>Source</Text>
          <Text bold>{item.SrcIP}</Text>
          <Text>Dest</Text>
          <Text bold>{item.DstIP}</Text>
        </HStack>
      )
    }
    icon={
      <Icon
        icon={faBan}
        color="red.400"
        size={props.size == 'xs' ? '8x' : '12x'}
      />
    }
    {...props}
  />
)

const Cards = {
  trigger: [
    {
      title: 'Date',
      description: 'Trigger on selected date and time',
      color: 'violet.300',
      icon: faClock,
      props: [
        { name: 'days', type: PropTypes.array },
        { name: 'from', type: PropTypes.string },
        { name: 'to', type: PropTypes.string }
      ]
    },
    {
      title: 'Incoming GET',
      description: 'Trigger this card by sending a GET request',
      color: 'red.400',
      icon: faBroadcastTower,
      props: [{ name: 'event', type: PropTypes.string }]
    }
  ],
  action: [
    {
      title: 'Block TCP',
      description: 'Block TCP for specified source and destination',
      color: 'red.400',
      icon: faBan,
      props: [
        {
          name: 'Protocol',
          value: 'TCP',
          hidden: true,
          type: PropTypes.string
        },
        { name: 'SrcIP', type: PropTypes.string },
        { name: 'DstIP', type: PropTypes.string }
      ]
    },
    {
      title: 'Block UDP',
      description: 'Block UDP for specified source and destination',
      color: 'warning.400',
      icon: faBan,
      props: [
        {
          name: 'Protocol',
          value: 'UDP',
          hidden: true,
          type: PropTypes.string
        },
        { name: 'SrcIP', type: PropTypes.string },
        { name: 'DstIP', type: PropTypes.string }
      ]
    }
  ]
}

export { FlowCard, Token, TriggerCardDate, ActionCardBlock, Cards }
export default FlowCard
