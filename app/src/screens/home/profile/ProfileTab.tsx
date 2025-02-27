import { RouteProp } from '@react-navigation/native';
import {
  createStackNavigator,
  StackNavigationProp,
} from '@react-navigation/stack';
import React from 'react';
import { ProfileTabScreenProps } from '../HomeScreen';
import EditScreen from './EditScreen';
import ProfileScreen from './ProfileScreen';

type ProfileTabStackParams = {
  Profile: undefined;
  Edit: undefined;
};

const Stack = createStackNavigator<ProfileTabStackParams>();

export type ProfileScreenProps = StackNavigationProp<
  ProfileTabStackParams,
  'Profile'
>;
export type ProfileEditScreenProps = StackNavigationProp<
  ProfileTabStackParams,
  'Edit'
>;

export type ProfileScreenParams = RouteProp<ProfileTabStackParams, 'Profile'>;

export default function ProfileTab(props: {
  navigation: ProfileTabScreenProps;
}) {
  return (
    <Stack.Navigator
      headerMode="none"
      screenOptions={{
        transitionSpec: {
          open: { animation: 'timing', config: { duration: 150 } },
          close: { animation: 'timing', config: { duration: 150 } },
        },
        cardStyleInterpolator: ({ current }) => ({
          cardStyle: {
            opacity: current.progress,
          },
        }),
      }}
    >
      <Stack.Screen
        name="Profile"
        component={ProfileScreen}
        listeners={{
          focus: () => {},
        }}
      />
      <Stack.Screen
        name="Edit"
        component={EditScreen}
        listeners={{
          focus: () => {},
        }}
      />
    </Stack.Navigator>
  );
}
